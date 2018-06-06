package root

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/kubernetes-incubator/kube-aws/awsconn"
	"github.com/kubernetes-incubator/kube-aws/cfnstack"
	controlplane "github.com/kubernetes-incubator/kube-aws/core/controlplane/cluster"
	controlplane_cfg "github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	etcd "github.com/kubernetes-incubator/kube-aws/core/etcd/cluster"
	network "github.com/kubernetes-incubator/kube-aws/core/network/cluster"
	nodepool "github.com/kubernetes-incubator/kube-aws/core/nodepool/cluster"
	nodepool_cfg "github.com/kubernetes-incubator/kube-aws/core/nodepool/config"
	"github.com/kubernetes-incubator/kube-aws/core/root/config"
	"github.com/kubernetes-incubator/kube-aws/core/root/defaults"
	"github.com/kubernetes-incubator/kube-aws/filereader/jsontemplate"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/kubernetes-incubator/kube-aws/model"
	"github.com/kubernetes-incubator/kube-aws/naming"
	"github.com/kubernetes-incubator/kube-aws/plugin/clusterextension"
	"github.com/kubernetes-incubator/kube-aws/plugin/pluginmodel"
	"github.com/tidwall/sjson"
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
		logger.Infof("Exporting %s\n", path)
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create directory \"%s\": %v", dir, err)
		}
		if err := ioutil.WriteFile(path, []byte(asset.Content), 0600); err != nil {
			return fmt.Errorf("Error writing %s : %v", path, err)
		}
		if strings.HasSuffix(path, "stack.json") && c.controlPlane.KMSKeyARN == "" {
			logger.Warnf("%s contains your TLS secrets!\n", path)
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
	Update(OperationTargets) (string, error)
	ValidateStack(...OperationTargets) (string, error)
	ValidateTemplates() error
	ControlPlane() *controlplane.Cluster
	Etcd() *etcd.Cluster
	Network() *network.Cluster
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
	session, err := awsconn.NewSessionFromRegion(cfg.Region, awsDebug)
	if err != nil {
		return nil, fmt.Errorf("failed to establish aws session: %v", err)
	}

	plugins := cfg.Plugins

	stackTemplateOpts := controlplane_cfg.StackTemplateOptions{
		AssetsDir:             opts.AssetsDir,
		ControllerTmplFile:    opts.ControllerTmplFile,
		EtcdTmplFile:          opts.EtcdTmplFile,
		StackTemplateTmplFile: opts.ControlPlaneStackTemplateTmplFile,
		PrettyPrint:           opts.PrettyPrint,
		S3URI:                 cfg.DeploymentSettings.S3URI,
		SkipWait:              opts.SkipWait,
	}

	netOpts := stackTemplateOpts
	netOpts.StackTemplateTmplFile = opts.NetworkStackTemplateTmplFile
	net, err := network.NewCluster(cfg.Cluster, netOpts, plugins, session)
	if err != nil {
		return nil, fmt.Errorf("failed to initizlie network stack: %v", err)
	}

	cpOpts := stackTemplateOpts
	cpOpts.StackTemplateTmplFile = opts.ControlPlaneStackTemplateTmplFile
	cp, err := controlplane.NewCluster(cfg.Cluster, cpOpts, plugins, session)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize control-plane stack: %v", err)
	}

	etcdOpts := stackTemplateOpts
	etcdOpts.StackTemplateTmplFile = opts.EtcdStackTemplateTmplFile
	etcd, err := etcd.NewCluster(cfg.Cluster, etcdOpts, plugins, session)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize etcd stack: %v", err)
	}

	nodePools := []*nodepool.Cluster{}
	for i, c := range cfg.NodePools {
		npOpts := nodepool_cfg.StackTemplateOptions{
			AssetsDir:             opts.AssetsDir,
			WorkerTmplFile:        opts.WorkerTmplFile,
			StackTemplateTmplFile: opts.NodePoolStackTemplateTmplFile,
			PrettyPrint:           opts.PrettyPrint,
			S3URI:                 cfg.DeploymentSettings.S3URI,
			SkipWait:              opts.SkipWait,
		}
		np, err := nodepool.NewCluster(c, npOpts, plugins, session)
		if err != nil {
			return nil, fmt.Errorf("failed to load node pool #%d: %v", i, err)
		}
		nodePools = append(nodePools, np)
	}

	extras := clusterextension.NewExtrasFromPlugins(plugins, cp.PluginConfigs)
	extra, err := extras.RootStack()
	if err != nil {
		return nil, fmt.Errorf("failed to load root stack extras from plugins: %v", err)
	}

	c := clusterImpl{
		opts:              opts,
		controlPlane:      cp,
		etcd:              etcd,
		network:           net,
		nodePools:         nodePools,
		session:           session,
		ExtraCfnResources: extra.Resources,
	}

	return c, nil
}

type clusterImpl struct {
	controlPlane      *controlplane.Cluster
	etcd              *etcd.Cluster
	network           *network.Cluster
	nodePools         []*nodepool.Cluster
	opts              options
	session           *session.Session
	ExtraCfnResources map[string]interface{}
}

func (c clusterImpl) ControlPlane() *controlplane.Cluster {
	return c.controlPlane
}

func (c clusterImpl) Etcd() *etcd.Cluster {
	return c.etcd
}

func (c clusterImpl) Network() *network.Cluster {
	return c.network
}

func (c clusterImpl) s3URI() string {
	return c.controlPlane.S3URI
}

func (c clusterImpl) NodePools() []*nodepool.Cluster {
	return c.nodePools
}

func (c clusterImpl) allOperationTargets() OperationTargets {
	names := []string{}
	for _, np := range c.nodePools {
		names = append(names, np.NodePoolName)
	}
	return AllOperationTargetsWith(names)
}

func (c clusterImpl) operationTargetsFromUserInput(opts []OperationTargets) OperationTargets {
	var targets OperationTargets
	if len(opts) > 0 && !opts[0].IsAll() {
		targets = OperationTargetsFromStringSlice(opts[0])
	} else {
		targets = c.allOperationTargets()
	}
	return targets
}

func (c clusterImpl) Create() error {
	cfSvc := cloudformation.New(c.session)

	assets, err := c.generateAssets(c.allOperationTargets())
	if err != nil {
		return err
	}

	err = c.uploadAssets(assets)
	if err != nil {
		return err
	}

	stackTemplateURL, err := c.extractRootStackTemplateURL(assets)
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

func (c clusterImpl) generateAssets(targets OperationTargets) (cfnstack.Assets, error) {
	logger.Infof("generating assets for %s\n", targets.String())
	var netAssets cfnstack.Assets
	if targets.IncludeNetwork() {
		netAssets = c.network.Assets()
	} else {
		netAssets = cfnstack.EmptyAssets()
	}

	var cpAssets cfnstack.Assets
	if targets.IncludeControlPlane() {
		cpAssets = c.controlPlane.Assets()
	} else {
		cpAssets = cfnstack.EmptyAssets()
	}

	var etcdAssets cfnstack.Assets
	if targets.IncludeEtcd() {
		etcdAssets = c.etcd.Assets()
	} else {
		etcdAssets = cfnstack.EmptyAssets()
	}

	var wAssets cfnstack.Assets
	wAssets = cfnstack.EmptyAssets()
	for _, np := range c.nodePools {
		if targets.IncludeWorker(np.NodePoolName) {
			wAssets = wAssets.Merge(np.Assets())
		}
	}

	nestedStacksAssets := netAssets.Merge(cpAssets).Merge(etcdAssets).Merge(wAssets)

	s3URI := fmt.Sprintf("%s/kube-aws/clusters/%s/exported/stacks",
		strings.TrimSuffix(c.s3URI(), "/"),
		c.controlPlane.ClusterName,
	)
	rootStackAssetsBuilder := cfnstack.NewAssetsBuilder(c.stackName(), s3URI, c.controlPlane.Region)

	var stackTemplate string
	// Do not update the root stack but update either controlplane or worker stack(s) only when specified so
	includeAll := targets.IncludeNetwork() && targets.IncludeEtcd() && targets.IncludeControlPlane()
	for _, np := range c.nodePools {
		includeAll = includeAll && targets.IncludeWorker(np.NodePoolName)
	}
	if includeAll {
		renderedTemplate, err := c.renderTemplateAsString()
		if err != nil {
			return nil, fmt.Errorf("failed to render template : %v", err)
		}
		stackTemplate = renderedTemplate
	} else {
		for _, target := range targets {
			logger.Infof("updating template url of %s\n", target)

			rootStackTemplate, err := c.getCurrentRootStackTemplate()
			if err != nil {
				return nil, fmt.Errorf("failed to render template : %v", err)
			}

			a, err := nestedStacksAssets.FindAssetByStackAndFileName(target, REMOTE_STACK_TEMPLATE_FILENAME)
			if err != nil {
				return nil, fmt.Errorf("failed to find assets for stack %s: %v", target, err)
			}

			nestedStackTemplateURL, err := a.URL()
			if err != nil {
				return nil, fmt.Errorf("failed to locate %s stack template url: %v", target, err)
			}

			stackTemplate, err = c.setNestedStackTemplateURL(rootStackTemplate, target, nestedStackTemplateURL)
			if err != nil {
				return nil, fmt.Errorf("failed to update stack template: %v", err)
			}
		}
	}
	rootStackAssetsBuilder.Add(REMOTE_STACK_TEMPLATE_FILENAME, stackTemplate)

	rootStackAssets := rootStackAssetsBuilder.Build()

	return nestedStacksAssets.Merge(rootStackAssets), nil
}

func (c clusterImpl) setNestedStackTemplateURL(template, stack string, url string) (string, error) {
	path := fmt.Sprintf("Resources.%s.Properties.TemplateURL", naming.FromStackToCfnResource(stack))
	return sjson.Set(template, path, url)
}

func (c clusterImpl) getCurrentRootStackTemplate() (string, error) {
	cfnSvc := cloudformation.New(c.session)
	byRootStackName := &cloudformation.GetTemplateInput{StackName: aws.String(c.stackName())}
	output, err := cfnSvc.GetTemplate(byRootStackName)
	if err != nil {
		return "", fmt.Errorf("failed to get current root stack template: %v", err)
	}
	return aws.StringValue(output.TemplateBody), nil
}

func (c clusterImpl) uploadAssets(assets cfnstack.Assets) error {
	s3Svc := s3.New(c.session)
	err := c.stackProvisioner().UploadAssets(s3Svc, assets)
	if err != nil {
		return fmt.Errorf("failed to upload assets: %v", err)
	}
	return nil
}

func (c clusterImpl) extractRootStackTemplateURL(assets cfnstack.Assets) (string, error) {
	asset, err := assets.FindAssetByStackAndFileName(c.stackName(), REMOTE_STACK_TEMPLATE_FILENAME)

	if err != nil {
		return "", fmt.Errorf("failed to find root stack template: %v", err)
	}

	return asset.URL()
}

func (c clusterImpl) Assets() (cfnstack.Assets, error) {
	return c.generateAssets(c.allOperationTargets())
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
		c.s3URI(),
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

func (c clusterImpl) Update(targets OperationTargets) (string, error) {
	cfSvc := cloudformation.New(c.session)

	assets, err := c.generateAssets(c.operationTargetsFromUserInput([]OperationTargets{targets}))
	if err != nil {
		return "", err
	}

	err = c.uploadAssets(assets)
	if err != nil {
		return "", err
	}

	templateUrl, err := c.extractRootStackTemplateURL(assets)
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
	if _, err := c.network.RenderStackTemplateAsString(); err != nil {
		return fmt.Errorf("failed to validate network template: %v", err)
	}
	if _, err := c.etcd.RenderStackTemplateAsString(); err != nil {
		return fmt.Errorf("failed to validate etcd template: %v", err)
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
func (c clusterImpl) ValidateStack(opts ...OperationTargets) (string, error) {
	reports := []string{}

	targets := c.operationTargetsFromUserInput(opts)

	assets, err := c.generateAssets(c.operationTargetsFromUserInput([]OperationTargets{targets}))
	if err != nil {
		return "", err
	}

	// Upload all the assets including stack templates and cloud-configs for all the stacks
	err = c.uploadAssets(assets)
	if err != nil {
		return "", err
	}

	rootStackTemplateURL, err := c.extractRootStackTemplateURL(assets)
	if err != nil {
		return "", err
	}

	r, err := c.stackProvisioner().ValidateStackAtURL(rootStackTemplateURL)
	if err != nil {
		return "", err
	}

	reports = append(reports, r)

	netReport, err := c.network.ValidateStack()
	if err != nil {
		return "", fmt.Errorf("failed to validate network: %v", err)
	}
	reports = append(reports, netReport)

	cpReport, err := c.controlPlane.ValidateStack()
	if err != nil {
		return "", fmt.Errorf("failed to validate control plane: %v", err)
	}
	reports = append(reports, cpReport)

	etcdReport, err := c.etcd.ValidateStack()
	if err != nil {
		return "", fmt.Errorf("failed to validate etcd plane: %v", err)
	}
	reports = append(reports, etcdReport)

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
	logger.Infof("Streaming filtered Journald logs for log group '%s'...\nNOTE: Due to high initial entropy, '.service' failures may occur during the early stages of booting.\n", c.controlPlane.ClusterName)
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
						logger.Infof("%s\t%s: \"%s\"\n", d, res.Hostname, res.Message)
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
	logger.Infof("Streaming CloudFormation events for the cluster '%s'...\n", c.controlPlane.ClusterName)
	return c.stackProvisioner().StreamEventsNested(q, cfSvc, c.controlPlane.ClusterName, c.controlPlane.ClusterName, time.Now())
}
