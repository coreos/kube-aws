package root

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/kubernetes-incubator/kube-aws/cfnstack"
	controlplane "github.com/kubernetes-incubator/kube-aws/core/controlplane/cluster"
	controlplane_cfg "github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	nodepool "github.com/kubernetes-incubator/kube-aws/core/nodepool/cluster"
	nodepool_cfg "github.com/kubernetes-incubator/kube-aws/core/nodepool/config"
	"github.com/kubernetes-incubator/kube-aws/core/root/config"
	"github.com/kubernetes-incubator/kube-aws/core/root/defaults"
	"github.com/kubernetes-incubator/kube-aws/filereader/jsontemplate"
	"github.com/kubernetes-incubator/kube-aws/model"
	"github.com/kubernetes-incubator/kube-aws/plugin/clusterextension"
	"github.com/kubernetes-incubator/kube-aws/plugin/pluginmodel"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	LOCAL_ROOT_STACK_TEMPLATE_PATH = defaults.RootStackTemplateTmplFile
	REMOTE_STACK_TEMPLATE_FILENAME = "stack.json"
)

func (c clusterImpl) Export() error {
	assets, err := c.Assets()

	if err != nil {
		return err
	}

	for _, asset := range assets.AsMap() {
		path := filepath.Join("exported", "stacks", asset.Path)
		fmt.Printf("Exporting %s\n", path)
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create directory \"%s\": %v", dir, err)
		}
		if err := ioutil.WriteFile(path, []byte(asset.Content), 0600); err != nil {
			return fmt.Errorf("Error writing %s : %v", path, err)
		}
		if strings.HasSuffix(path, "stack.json") && c.controlPlane.KMSKeyARN == "" {
			fmt.Printf("BEWARE: %s contains your TLS secrets!\n", path)
		}
	}
	return nil
}

func (c clusterImpl) EstimateCost() ([]string, error) {

	cfSvc := cloudformation.New(c.session)
	var urls []string

	controlPlaneTemplate, err := c.controlPlane.RenderStackTemplateAsString()

	if err != nil {
		return nil, fmt.Errorf("failed to render control plane template %v", err)
	}

	controlPlaneCost, err := c.stackProvisioner().EstimateTemplateCost(cfSvc, controlPlaneTemplate, nil)

	if err != nil {
		return nil, fmt.Errorf("failed to estimate cost for control plane %v", err)
	}

	urls = append(urls, *controlPlaneCost.Url)

	for i, p := range c.nodePools {
		nodePoolsTemplate, err := p.RenderStackTemplateAsString()

		if err != nil {
			return nil, fmt.Errorf("failed to render node pool #%d template: %v", i, err)
		}

		nodePoolsCost, err := c.stackProvisioner().EstimateTemplateCost(cfSvc, nodePoolsTemplate, []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String("ControlPlaneStackName"),
				ParameterValue: aws.String("fake-name"),
			},
		})

		if err != nil {
			return nil, fmt.Errorf("failed to estimate cost for node pool #%d %v", i, err)
		}

		urls = append(urls, *nodePoolsCost.Url)
	}

	return urls, nil

}

type Cluster interface {
	Assets() (cfnstack.Assets, error)
	Create() error
	Export() error
	EstimateCost() ([]string, error)
	Info() (*Info, error)
	Update() (string, error)
	ValidateStack() (string, error)
	ValidateTemplates() error
	ControlPlane() *controlplane.Cluster
	NodePools() []*nodepool.Cluster
	RenderStackTemplateAsString() (string, error)
}

func ClusterFromFile(configPath string, opts options, awsDebug bool) (Cluster, error) {
	cfg, err := config.ConfigFromFile(configPath)
	if err != nil {
		return nil, err
	}
	return ClusterFromConfig(cfg, opts, awsDebug)
}

func ClusterFromConfig(cfg *config.Config, opts options, awsDebug bool) (Cluster, error) {
	plugins := cfg.Plugins

	cpOpts := controlplane_cfg.StackTemplateOptions{
		AssetsDir:             opts.AssetsDir,
		ControllerTmplFile:    opts.ControllerTmplFile,
		EtcdTmplFile:          opts.EtcdTmplFile,
		StackTemplateTmplFile: opts.ControlPlaneStackTemplateTmplFile,
		PrettyPrint:           opts.PrettyPrint,
		S3URI:                 opts.S3URI,
		SkipWait:              opts.SkipWait,
	}
	cp, err := controlplane.NewCluster(cfg.Cluster, cpOpts, plugins, awsDebug)
	if err != nil {
		return nil, err
	}
	nodePools := []*nodepool.Cluster{}
	for i, c := range cfg.NodePools {
		npOpts := nodepool_cfg.StackTemplateOptions{
			AssetsDir:             opts.AssetsDir,
			WorkerTmplFile:        opts.WorkerTmplFile,
			StackTemplateTmplFile: opts.NodePoolStackTemplateTmplFile,
			PrettyPrint:           opts.PrettyPrint,
			S3URI:                 opts.S3URI,
			SkipWait:              opts.SkipWait,
		}
		np, err := nodepool.NewCluster(c, npOpts, plugins, awsDebug)
		if err != nil {
			return nil, fmt.Errorf("failed to load node pool #%d: %v", i, err)
		}
		nodePools = append(nodePools, np)
	}
	awsConfig := aws.NewConfig().
		WithRegion(cfg.Region.String()).
		WithCredentialsChainVerboseErrors(true)

	if awsDebug {
		awsConfig = awsConfig.WithLogLevel(aws.LogDebug)
	}

	session, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to establish aws session: %v", err)
	}

	extras := clusterextension.NewExtrasFromPlugins(plugins, cp.PluginConfigs)
	extra, err := extras.RootStack()
	if err != nil {
		return nil, fmt.Errorf("failed to load root stack extras from plugins: %v", err)
	}

	c := clusterImpl{
		opts:              opts,
		controlPlane:      cp,
		nodePools:         nodePools,
		session:           session,
		ExtraCfnResources: extra.Resources,
	}

	return c, nil
}

type clusterImpl struct {
	controlPlane      *controlplane.Cluster
	nodePools         []*nodepool.Cluster
	opts              options
	session           *session.Session
	ExtraCfnResources map[string]interface{}
}

func (c clusterImpl) ControlPlane() *controlplane.Cluster {
	return c.controlPlane
}

func (c clusterImpl) NodePools() []*nodepool.Cluster {
	return c.nodePools
}

func (c clusterImpl) Create() error {
	cfSvc := cloudformation.New(c.session)

	stackTemplateURL, err := c.prepareTemplateWithAssets()
	if err != nil {
		return err
	}

	q := make(chan struct{}, 1)
	defer func() { q <- struct{}{} }()

	if c.controlPlane.CloudWatchLogging.Enabled && c.controlPlane.CloudWatchLogging.LocalStreaming.Enabled {
		go streamJournaldLogs(c, q)
	}

	if c.controlPlane.CloudFormationStreaming {
		go streamStackEvents(c, cfSvc, q)
	}

	return c.stackProvisioner().CreateStackAtURLAndWait(cfSvc, stackTemplateURL)
}

func (c clusterImpl) Info() (*Info, error) {
	// TODO Cleaner way to obtain this dependency
	cpConfig, err := c.controlPlane.Cluster.Config([]*pluginmodel.Plugin{})
	if err != nil {
		return nil, err
	}

	describer := NewClusterDescriber(c.controlPlane.ClusterName, c.stackName(), cpConfig, c.session)
	return describer.Info()
}

func (c clusterImpl) prepareTemplateWithAssets() (string, error) {
	assets, err := c.Assets()

	if err != nil {
		return "", err
	}

	s3Svc := s3.New(c.session)
	err = c.stackProvisioner().UploadAssets(s3Svc, assets)
	if err != nil {
		return "", err
	}

	asset, err := assets.FindAssetByStackAndFileName(c.stackName(), REMOTE_STACK_TEMPLATE_FILENAME)

	if err != nil {
		return "", fmt.Errorf("failed to prepare template with assets: %v", err)
	}

	return asset.URL()
}

func (c clusterImpl) Assets() (cfnstack.Assets, error) {
	stackTemplate, err := c.renderTemplateAsString()
	if err != nil {
		return nil, fmt.Errorf("Error while rendering template : %v", err)
	}
	s3URI := fmt.Sprintf("%s/kube-aws/clusters/%s/exported/stacks",
		strings.TrimSuffix(c.opts.S3URI, "/"),
		c.controlPlane.ClusterName,
	)

	assetsBuilder := cfnstack.NewAssetsBuilder(c.stackName(), s3URI, c.controlPlane.Region)
	assetsBuilder.Add(REMOTE_STACK_TEMPLATE_FILENAME, stackTemplate)
	assets := assetsBuilder.Build()

	cpAssets := c.controlPlane.Assets()
	assets = assets.Merge(cpAssets)

	for _, np := range c.nodePools {
		a := np.Assets()
		assets = assets.Merge(a)
	}

	return assets, nil
}

func (c clusterImpl) templatePath() string {
	return c.opts.RootStackTemplateTmplFile
}

func (c clusterImpl) templateParams() TemplateParams {
	params := newTemplateParams(c)
	return params
}

func (c clusterImpl) RenderStackTemplateAsString() (string, error) {
	return c.renderTemplateAsString()
}

func (c clusterImpl) renderTemplateAsString() (string, error) {
	template, err := jsontemplate.GetString(c.templatePath(), c.templateParams(), c.opts.PrettyPrint)
	if err != nil {
		return "", err
	}
	return template, nil
}

func (c clusterImpl) stackProvisioner() *cfnstack.Provisioner {
	stackPolicyBody := `{
  "Statement" : [
    {
       "Effect" : "Allow",
       "Principal" : "*",
       "Action" : "Update:*",
       "Resource" : "*"
     }
  ]
}`
	return cfnstack.NewProvisioner(
		c.stackName(),
		c.tags(),
		c.opts.S3URI,
		c.controlPlane.Region,
		stackPolicyBody,
		c.session,
		c.controlPlane.CloudFormation.RoleARN,
	)
}

func (c clusterImpl) stackName() string {
	return c.controlPlane.Cluster.ClusterName
}

func (c clusterImpl) tags() map[string]string {
	return c.controlPlane.Cluster.StackTags
}

func (c clusterImpl) Update() (string, error) {
	cfSvc := cloudformation.New(c.session)

	templateUrl, err := c.prepareTemplateWithAssets()
	if err != nil {
		return "", err
	}

	q := make(chan struct{}, 1)
	defer func() { q <- struct{}{} }()

	if c.controlPlane.CloudWatchLogging.Enabled && c.controlPlane.CloudWatchLogging.LocalStreaming.Enabled {
		go streamJournaldLogs(c, q)
	}

	if c.controlPlane.CloudFormationStreaming {
		go streamStackEvents(c, cfSvc, q)
	}

	return c.stackProvisioner().UpdateStackAtURLAndWait(cfSvc, templateUrl)
}

func (c clusterImpl) ValidateTemplates() error {
	_, err := c.renderTemplateAsString()
	if err != nil {
		return fmt.Errorf("failed to validate template: %v", err)
	}
	if _, err := c.controlPlane.RenderStackTemplateAsString(); err != nil {
		return fmt.Errorf("failed to validate control plane template: %v", err)
	}
	for i, p := range c.nodePools {
		if _, err := p.RenderStackTemplateAsString(); err != nil {
			return fmt.Errorf("failed to validate node pool #%d template: %v", i, err)
		}
	}
	return nil
}

// ValidateStack validates all the CloudFormation stack templates already uploaded to S3
func (c clusterImpl) ValidateStack() (string, error) {
	reports := []string{}

	// Upload all the assets including stack templates and cloud-configs for all the stacks
	rootStackTemplateURL, err := c.prepareTemplateWithAssets()
	if err != nil {
		return "", err
	}

	r, err := c.stackProvisioner().ValidateStackAtURL(rootStackTemplateURL)
	if err != nil {
		return "", err
	}

	reports = append(reports, r)

	cpReport, err := c.controlPlane.ValidateStack()
	if err != nil {
		return "", fmt.Errorf("failed to validate control plane: %v", err)
	}
	reports = append(reports, cpReport)

	for i, p := range c.nodePools {
		npReport, err := p.ValidateStack()
		if err != nil {
			return "", fmt.Errorf("failed to validate node pool #%d: %v", i, err)
		}
		reports = append(reports, npReport)
	}

	return strings.Join(reports, "\n"), nil
}

func streamJournaldLogs(c clusterImpl, q chan struct{}) error {
	fmt.Printf("Streaming filtered Journald logs for log group '%s'...\nNOTE: Due to high initial entropy, '.service' failures may occur during the early stages of booting.\n", c.controlPlane.ClusterName)
	cwlSvc := cloudwatchlogs.New(c.session)
	s := time.Now().Unix() * 1E3
	t := s
	in := cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:  &c.controlPlane.ClusterName,
		FilterPattern: &c.controlPlane.CloudWatchLogging.LocalStreaming.Filter,
		StartTime:     &s}
	ms := make(map[string]int64)

	for {
		select {
		case <-q:
			return nil
		case <-time.After(1 * time.Second):
			out, err := cwlSvc.FilterLogEvents(&in)
			if err != nil {
				continue
			}
			if len(out.Events) > 1 {
				s = *out.Events[len(out.Events)-1].Timestamp
				for _, event := range out.Events {
					if *event.Timestamp > ms[*event.Message]+c.controlPlane.CloudWatchLogging.LocalStreaming.Interval() {
						ms[*event.Message] = *event.Timestamp
						res := model.SystemdMessageResponse{}
						json.Unmarshal([]byte(*event.Message), &res)
						s := int(((*event.Timestamp) - t) / 1E3)
						d := fmt.Sprintf("+%.2d:%.2d:%.2d", s/3600, (s/60)%60, s%60)
						fmt.Printf("%s\t%s: \"%s\"\n", d, res.Hostname, res.Message)
					}
				}
			}
			in = cloudwatchlogs.FilterLogEventsInput{
				LogGroupName:  &c.controlPlane.ClusterName,
				FilterPattern: &c.controlPlane.CloudWatchLogging.LocalStreaming.Filter,
				NextToken:     out.NextToken,
				StartTime:     &s}
		}
	}
}

// streamStackEvents streams all the events from the root, the control-plane, and worker node pool stacks using StreamEventsNested
func streamStackEvents(c clusterImpl, cfSvc *cloudformation.CloudFormation, q chan struct{}) error {
	fmt.Printf("Streaming CloudFormation events for the cluster '%s'...\n", c.controlPlane.ClusterName)
	return c.stackProvisioner().StreamEventsNested(q, cfSvc, c.controlPlane.ClusterName, c.controlPlane.ClusterName, time.Now())
}
