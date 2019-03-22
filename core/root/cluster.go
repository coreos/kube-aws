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
	"github.com/kubernetes-incubator/kube-aws/core/root/config"
	"github.com/kubernetes-incubator/kube-aws/core/root/defaults"
	"github.com/kubernetes-incubator/kube-aws/credential"
	"github.com/kubernetes-incubator/kube-aws/filereader/jsontemplate"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/kubernetes-incubator/kube-aws/naming"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"github.com/kubernetes-incubator/kube-aws/pkg/model"
	"github.com/kubernetes-incubator/kube-aws/plugin/clusterextension"
	"github.com/tidwall/sjson"
)

const (
	LOCAL_ROOT_STACK_TEMPLATE_PATH = defaults.RootStackTemplateTmplFile
	REMOTE_STACK_TEMPLATE_FILENAME = "stack.json"
)

func (cl Cluster) Export() error {
	assets, err := cl.EnsureAllAssetsGenerated()

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
		if strings.HasSuffix(path, "stack.json") && cl.Cfg.KMSKeyARN == "" {
			logger.Warnf("%s contains your TLS secrets!\n", path)
		}
	}
	return nil
}

func (cl Cluster) EstimateCost() ([]string, error) {

	cfSvc := cloudformation.New(cl.session)
	var urls []string

	controlPlaneTemplate, err := cl.controlPlaneStack.RenderStackTemplateAsString()

	if err != nil {
		return nil, fmt.Errorf("failed to render control plane template %v", err)
	}

	controlPlaneCost, err := cl.stackProvisioner().EstimateTemplateCost(cfSvc, controlPlaneTemplate, nil)

	if err != nil {
		return nil, fmt.Errorf("failed to estimate cost for control plane %v", err)
	}

	urls = append(urls, *controlPlaneCost.Url)

	for i, p := range cl.nodePoolStacks {
		nodePoolsTemplate, err := p.RenderStackTemplateAsString()

		if err != nil {
			return nil, fmt.Errorf("failed to render node pool #%d template: %v", i, err)
		}

		nodePoolsCost, err := cl.stackProvisioner().EstimateTemplateCost(cfSvc, nodePoolsTemplate, []*cloudformation.Parameter{
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

type DiffResult struct {
	Target string
	diff   string
}

func (r *DiffResult) String() string {
	return r.diff
}

type Cluster struct {
	controlPlaneStack *model.Stack
	etcdStack         *model.Stack
	networkStack      *model.Stack
	nodePoolStacks    []*model.Stack

	ExtraCfnResources map[string]interface{}
	ExtraCfnTags      map[string]interface{}
	ExtraCfnOutputs   map[string]interface{}

	opts     options
	session  *session.Session
	Context  *model.Context
	extras   clusterextension.ClusterExtension
	Cfg      *config.Config
	loaded   bool
	awsDebug bool
}

func LoadClusterFromFile(configPath string, opts options, awsDebug bool) (*Cluster, error) {
	cfg, err := config.ConfigFromFile(configPath)
	if err != nil {
		return nil, err
	}
	return CompileClusterFromConfig(cfg, opts, awsDebug)
}

func CompileClusterFromFile(configPath string, opts options, awsDebug bool) (*Cluster, error) {
	cfg, err := config.ConfigFromFile(configPath)
	if err != nil {
		return nil, err
	}
	return CompileClusterFromConfig(cfg, opts, awsDebug)
}

func CompileClusterFromConfig(cfg *config.Config, opts options, awsDebug bool) (*Cluster, error) {
	session, err := awsconn.NewSessionFromRegion(cfg.Region, awsDebug)
	if err != nil {
		return nil, fmt.Errorf("failed to establish aws session: %v", err)
	}

	return &Cluster{Cfg: cfg, opts: opts, awsDebug: awsDebug, extras: *cfg.Extras, session: session}, nil
}

func (cl *Cluster) context() *model.Context {
	if cl.Context == nil {
		cfn := cloudformation.New(cl.session)
		cl.Context = &model.Context{
			Session:                cl.session,
			ProvidedCFInterrogator: cfn,
			StackTemplateGetter:    cfn,
		}
	}
	return cl.Context
}

func (cl *Cluster) ensureNestedStacksLoaded() error {
	if cl.loaded {
		return nil
	}
	cl.loaded = true

	rootcfg := cl.Cfg
	opts := cl.opts
	plugins := cl.Cfg.Plugins

	extras := clusterextension.NewExtrasFromPlugins(plugins, rootcfg.PluginConfigs)

	stackTemplateOpts := api.StackTemplateOptions{
		AssetsDir:             opts.AssetsDir,
		ControllerTmplFile:    opts.ControllerTmplFile,
		EtcdTmplFile:          opts.EtcdTmplFile,
		StackTemplateTmplFile: opts.ControlPlaneStackTemplateTmplFile,
		PrettyPrint:           opts.PrettyPrint,
		S3URI:                 rootcfg.DeploymentSettings.S3URI,
		SkipWait:              opts.SkipWait,
	}

	cfg := cl.Cfg.Config

	assetsConfig, err := cl.context().LoadCredentials(cfg, stackTemplateOpts)
	if err != nil {
		return fmt.Errorf("failed initializing credentials: %v", err)
	}

	netOpts := stackTemplateOpts
	netOpts.StackTemplateTmplFile = opts.NetworkStackTemplateTmplFile

	cpOpts := stackTemplateOpts
	cpOpts.StackTemplateTmplFile = opts.ControlPlaneStackTemplateTmplFile
	cp, err := model.NewControlPlaneStack(cfg, cpOpts, extras, assetsConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize control-plane stack: %v", err)
	}

	etcdOpts := stackTemplateOpts
	etcdOpts.StackTemplateTmplFile = opts.EtcdStackTemplateTmplFile
	etcd, err := model.NewEtcdStack(cfg, etcdOpts, extras, assetsConfig, cl.context())
	if err != nil {
		return fmt.Errorf("failed to initialize etcd stack: %v", err)
	}

	nodePools := []*model.Stack{}
	for i, c := range cfg.NodePools {
		npOpts := api.StackTemplateOptions{
			AssetsDir:             opts.AssetsDir,
			WorkerTmplFile:        opts.WorkerTmplFile,
			StackTemplateTmplFile: opts.NodePoolStackTemplateTmplFile,
			PrettyPrint:           opts.PrettyPrint,
			S3URI:                 cfg.DeploymentSettings.S3URI,
			SkipWait:              opts.SkipWait,
		}
		npCfg, err := model.NodePoolCompile(c, cfg)
		if err != nil {
			return fmt.Errorf("failed initializing worker node pool: %v", err)
		}
		npExtras := extras
		npExtras.Configs = npCfg.Plugins
		np, err := model.NewWorkerStack(cfg, npCfg, npOpts, npExtras, assetsConfig)
		if err != nil {
			return fmt.Errorf("failed to load node pool #%d: %v", i, err)
		}
		nodePools = append(nodePools, np)
	}

	net, err := model.NewNetworkStack(cfg, nodePools, netOpts, extras, assetsConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize network stack: %v", err)
	}

	cl.etcdStack = etcd
	cl.controlPlaneStack = cp
	cl.networkStack = net
	cl.nodePoolStacks = nodePools

	extra, err := cl.extras.RootStack(cfg)
	if err != nil {
		return fmt.Errorf("failed to load root stack extras from plugins: %v", err)
	}

	cl.ExtraCfnResources = extra.Resources
	cl.ExtraCfnTags = extra.Tags
	cl.ExtraCfnOutputs = extra.Outputs

	return nil
}

func (cl *Cluster) GenerateAssetsOnDisk(dir string, opts credential.GeneratorOptions) (*credential.RawAssetsOnDisk, error) {
	a, err := model.GenerateAssetsOnDisk(cl.session, cl.Cfg.Config, dir, opts)
	if err != nil {
		return nil, err
	}
	kmsConfig := credential.NewKMSConfig(cl.Cfg.KMSKeyARN, nil, cl.session)
	enc := kmsConfig.Encryptor()
	p := credential.NewProtectedPKI(enc)
	specs := cl.extras.KeyPairSpecs()
	if err := p.EnsureKeyPairsCreated(specs); err != nil {
		return nil, err
	}
	return a, nil
}

func (cl *Cluster) ControlPlane() *model.Stack {
	return cl.controlPlaneStack
}

func (cl *Cluster) Etcd() *model.Stack {
	return cl.etcdStack
}

func (cl *Cluster) Network() *model.Stack {
	return cl.networkStack
}

func (cl *Cluster) s3URI() string {
	return cl.controlPlaneStack.S3URI
}

func (cl *Cluster) operationTargetNames() []string {
	return []string{
		cl.controlPlaneStack.Config.ControlPlaneStackName(),
		cl.networkStack.Config.NetworkStackName(),
		cl.etcdStack.Config.EtcdStackName(),
	}
}

func (cl *Cluster) NodePools() []*model.Stack {
	return cl.nodePoolStacks
}

func (cl *Cluster) allOperationTargets() OperationTargets {
	names := []string{}
	for _, np := range cl.nodePoolStacks {
		names = append(names, np.StackName)
	}
	return AllOperationTargetsWith(names, cl.operationTargetNames())
}

func (cl *Cluster) operationTargetsFromUserInput(opts []OperationTargets) OperationTargets {
	var targets OperationTargets
	if len(opts) > 0 && !opts[0].IsAll() {
		targets = OperationTargetsFromStringSlice(opts[0])
	} else {
		targets = cl.allOperationTargets()
	}
	return targets
}

type renderer interface {
	RenderStackTemplateAsString() (string, error)
}

type diffSetting struct {
	stackName      string
	renderer       renderer
	userdata       *api.UserData
	launchConfName string
}

func (cl *Cluster) Diff(opts OperationTargets, context int) ([]*DiffResult, error) {
	if err := cl.ensureNestedStacksLoaded(); err != nil {
		return nil, err
	}

	cfnSvc := cloudformation.New(cl.session)
	s3Svc := s3.New(cl.session)

	mappings := map[string]diffSetting{}

	isAll := opts.IsAll()
	includeAll := opts.IncludeAll(cl)
	for _, np := range cl.nodePoolStacks {
		includeAll = includeAll && opts.IncludeWorker(np.StackName)
	}
	if isAll || includeAll {
		mappings["root"] = diffSetting{cl.stackName(), cl, nil, ""}
	}

	if isAll || opts.IncludeNetwork(cl.networkStack.Config.NetworkStackName()) {
		stackName, err := getNestedStackName(cfnSvc, cl.stackName(), cl.networkStack.NestedStackName())
		if err != nil {
			return nil, err
		}
		mappings["network"] = diffSetting{stackName, cl.networkStack, nil, ""}
	}

	staticEtcdIndex := 0
	if isAll || opts.IncludeEtcd(cl.etcdStack.Config.EtcdStackName()) {
		stackName, err := getNestedStackName(cfnSvc, cl.stackName(), cl.etcdStack.NestedStackName())
		if err != nil {
			return nil, err
		}
		mappings["etcd"] = diffSetting{stackName, cl.etcdStack, cl.etcdStack.GetUserData("Etcd"), cl.etcdStack.Config.EtcdNodes[staticEtcdIndex].LaunchConfigurationLogicalName()}
	}

	if isAll || opts.IncludeControlPlane(cl.controlPlaneStack.Config.ControlPlaneStackName()) {
		stackName, err := getNestedStackName(cfnSvc, cl.stackName(), cl.controlPlaneStack.NestedStackName())
		if err != nil {
			return nil, err
		}
		mappings["controller"] = diffSetting{stackName, cl.controlPlaneStack, cl.controlPlaneStack.GetUserData("Controller"), cl.controlPlaneStack.Config.Controller.LaunchConfigurationLogicalName()}
	}

	for _, np := range cl.nodePoolStacks {
		if opts.IncludeWorker(np.StackName) {
			stackName, err := getNestedStackName(cfnSvc, cl.stackName(), np.NestedStackName())
			if err != nil {
				return nil, err
			}
			id := fmt.Sprintf("worker-%s", np.StackName)
			mappings[id] = diffSetting{stackName, np, np.GetUserData("Worker"), np.NodePoolConfig.WorkerNodePool.LaunchConfigurationLogicalName()}
		}
	}

	diffResults := []*DiffResult{}

	for id, setting := range mappings {
		currentStack, err := getStackTemplate(cfnSvc, setting.stackName)
		if err != nil {
			return nil, fmt.Errorf("failed to obtain %s stack template: %v", id, err)
		}
		desiredStack, err := setting.renderer.RenderStackTemplateAsString()
		if err != nil {
			return nil, fmt.Errorf("failed to render %s stack template: %v", id, err)
		}

		stackDiffOutput, err := diffJson(currentStack, desiredStack, context)
		if err != nil {
			return nil, err
		}
		stackDiffSummary := &DiffResult{fmt.Sprintf("%s-stack", id), stackDiffOutput}
		diffResults = append(diffResults, stackDiffSummary)

		if len(stackDiffOutput) > 0 && setting.userdata != nil {
			currentInsScriptUserdata, err := getInstanceScriptUserdata(currentStack, setting.launchConfName)
			if err != nil {
				return nil, fmt.Errorf("failed to obtain %s instance userdata template: %v", id, err)
			}
			desiredInsScriptUserdata, err := setting.userdata.Parts["instance-script"].Template(map[string]interface{}{"etcdIndex": staticEtcdIndex})
			if err != nil {
				return nil, fmt.Errorf("failed to render %s instance userdata template: %v", id, err)
			}

			insScriptUserdataDiffOutput, err := diffText(currentInsScriptUserdata, desiredInsScriptUserdata, context)
			if err != nil {
				return nil, err
			}
			insScriptUserdataDiffSummary := &DiffResult{fmt.Sprintf("%s-userdata-instance-script", id), insScriptUserdataDiffOutput}
			diffResults = append(diffResults, insScriptUserdataDiffSummary)

			currentInsUserdata, err := getInstanceUserdataJson(currentStack, setting.launchConfName)
			if err != nil {
				return nil, fmt.Errorf("failed to obtain %s instance userdata template: %v", id, err)
			}
			desiredInsUserdata, err := setting.userdata.Parts["instance"].Template(map[string]interface{}{"etcdIndex": staticEtcdIndex})
			if err != nil {
				return nil, fmt.Errorf("failed to render %s instance userdata template: %v", id, err)
			}

			insUserdataDiffOutput, err := diffJson(currentInsUserdata, desiredInsUserdata, context)
			if err != nil {
				return nil, err
			}
			insUserdataDiffSummary := &DiffResult{fmt.Sprintf("%s-userdata-instance", id), insUserdataDiffOutput}
			diffResults = append(diffResults, insUserdataDiffSummary)

			{
				currentS3Userdata, err := getS3Userdata(s3Svc, currentInsUserdata)
				if err != nil {
					return nil, fmt.Errorf("failed to obtain %s s3 userdata template: %v", id, err)
				}
				desiredS3Userdata, err := setting.userdata.Parts["s3"].Template(map[string]interface{}{"etcdIndex": staticEtcdIndex})
				if err != nil {
					return nil, fmt.Errorf("failed to render %s s3 userdata template: %v", id, err)
				}

				s3UserdataDiffOutput, err := diffText(currentS3Userdata, desiredS3Userdata, context)
				if err != nil {
					return nil, err
				}
				s3UserdataDiffSummary := &DiffResult{fmt.Sprintf("%s-userdata-s3", id), s3UserdataDiffOutput}
				diffResults = append(diffResults, s3UserdataDiffSummary)
			}
		}
	}

	return diffResults, nil
}

// remove with legacy up command
func (cl *Cluster) LegacyCreate() error {
	cfSvc := cloudformation.New(cl.session)
	return cl.create(cfSvc)
}

func (cl *Cluster) create(cfSvc *cloudformation.CloudFormation) error {
	if err := cl.ensureNestedStacksLoaded(); err != nil {
		return err
	}

	assets, err := cl.EnsureAllAssetsGenerated()

	err = cl.uploadAssets(assets)
	if err != nil {
		return err
	}

	stackTemplateURL, err := cl.extractRootStackTemplateURL(assets)
	if err != nil {
		return err
	}

	q := make(chan struct{}, 1)
	defer func() { q <- struct{}{} }()

	if cl.controlPlaneStack.Config.CloudWatchLogging.Enabled && cl.controlPlaneStack.Config.CloudWatchLogging.LocalStreaming.Enabled {
		go streamJournaldLogs(cl, q)
	}

	if cl.controlPlaneStack.Config.CloudFormationStreaming {
		go streamStackEvents(cl, cfSvc, q)
	}

	return cl.stackProvisioner().CreateStackAtURLAndWait(cfSvc, stackTemplateURL)
}

func (cl *Cluster) Info() (*Info, error) {
	if err := cl.ensureNestedStacksLoaded(); err != nil {
		return nil, err
	}

	describer := NewClusterDescriber(cl.controlPlaneStack.ClusterName, cl.stackName(), cl.Cfg.Config, cl.session)
	return describer.Info()
}

func (cl *Cluster) generateAssets(targets OperationTargets) (cfnstack.Assets, error) {
	logger.Infof("generating assets for %s\n", targets.String())
	var netAssets cfnstack.Assets
	if targets.IncludeNetwork(cl.networkStack.Config.NetworkStackName()) {
		netAssets = cl.networkStack.Assets()
	} else {
		netAssets = cfnstack.EmptyAssets()
	}

	var cpAssets cfnstack.Assets
	if targets.IncludeControlPlane(cl.controlPlaneStack.Config.ControlPlaneStackName()) {
		cpAssets = cl.controlPlaneStack.Assets()
	} else {
		cpAssets = cfnstack.EmptyAssets()
	}

	var etcdAssets cfnstack.Assets
	if targets.IncludeEtcd(cl.etcdStack.Config.EtcdStackName()) {
		etcdAssets = cl.etcdStack.Assets()
	} else {
		etcdAssets = cfnstack.EmptyAssets()
	}

	var wAssets cfnstack.Assets
	wAssets = cfnstack.EmptyAssets()
	for _, np := range cl.nodePoolStacks {
		if targets.IncludeWorker(np.StackName) {
			wAssets = wAssets.Merge(np.Assets())
		}
	}

	nestedStacksAssets := netAssets.Merge(cpAssets).
		Merge(etcdAssets).
		Merge(wAssets)

	s3URI := fmt.Sprintf("%s/kube-aws/clusters/%s/exported/stacks",
		strings.TrimSuffix(cl.s3URI(), "/"),
		cl.controlPlaneStack.ClusterName,
	)
	rootStackAssetsBuilder, err := cfnstack.NewAssetsBuilder(cl.stackName(), s3URI, cl.controlPlaneStack.Region)
	if err != nil {
		return nil, err
	}

	var stackTemplate string
	// Do not update the root stack but update either controlplane or worker stack(s) only when specified so
	includeAll := targets.IncludeAll(cl)
	for _, np := range cl.nodePoolStacks {
		includeAll = includeAll && targets.IncludeWorker(np.StackName)
	}
	if includeAll {
		renderedTemplate, err := cl.renderTemplateAsString()
		if err != nil {
			return nil, fmt.Errorf("failed to render template : %v", err)
		}
		stackTemplate = renderedTemplate
	} else {
		for _, target := range targets {
			logger.Infof("updating template url of %s\n", target)

			rootStackTemplate, err := cl.getCurrentRootStackTemplate()
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

			stackTemplate, err = cl.setNestedStackTemplateURL(rootStackTemplate, target, nestedStackTemplateURL)
			if err != nil {
				return nil, fmt.Errorf("failed to update stack template: %v", err)
			}
		}
	}
	rootStackAssetsBuilder.Add(REMOTE_STACK_TEMPLATE_FILENAME, stackTemplate)

	rootStackAssets := rootStackAssetsBuilder.Build()

	return nestedStacksAssets.Merge(rootStackAssets), nil
}

func (cl *Cluster) setNestedStackTemplateURL(template, stack string, url string) (string, error) {
	path := fmt.Sprintf("Resources.%s.Properties.TemplateURL", naming.FromStackToCfnResource(stack))
	return sjson.Set(template, path, url)
}

func (cl *Cluster) getCurrentRootStackTemplate() (string, error) {
	cfnSvc := cl.context().StackTemplateGetter
	byRootStackName := &cloudformation.GetTemplateInput{StackName: aws.String(cl.stackName())}
	output, err := cfnSvc.GetTemplate(byRootStackName)
	if err != nil {
		return "", fmt.Errorf("failed to get current root stack template: %v", err)
	}
	return aws.StringValue(output.TemplateBody), nil
}

func (cl *Cluster) uploadAssets(assets cfnstack.Assets) error {
	s3Svc := s3.New(cl.session)
	err := cl.stackProvisioner().UploadAssets(s3Svc, assets)
	if err != nil {
		return fmt.Errorf("failed to upload assets: %v", err)
	}
	return nil
}

func (cl *Cluster) extractRootStackTemplateURL(assets cfnstack.Assets) (string, error) {
	asset, err := assets.FindAssetByStackAndFileName(cl.stackName(), REMOTE_STACK_TEMPLATE_FILENAME)

	if err != nil {
		return "", fmt.Errorf("failed to find root stack template: %v", err)
	}

	return asset.URL()
}

func (cl *Cluster) EnsureAllAssetsGenerated() (cfnstack.Assets, error) {
	if err := cl.ensureNestedStacksLoaded(); err != nil {
		return nil, err
	}

	return cl.generateAssets(cl.allOperationTargets())
}

func (cl *Cluster) templatePath() string {
	return cl.opts.RootStackTemplateTmplFile
}

func (cl *Cluster) templateParams() TemplateParams {
	params := newTemplateParams(cl)
	return params
}

func (cl *Cluster) RenderStackTemplateAsString() (string, error) {
	return cl.renderTemplateAsString()
}

func (cl *Cluster) renderTemplateAsString() (string, error) {
	template, err := jsontemplate.GetString(cl.templatePath(), cl.templateParams(), cl.opts.PrettyPrint)
	if err != nil {
		return "", err
	}
	return template, nil
}

func (cl *Cluster) stackProvisioner() *cfnstack.Provisioner {
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
		cl.stackName(),
		cl.tags(),
		cl.s3URI(),
		cl.controlPlaneStack.Region,
		stackPolicyBody,
		cl.session,
		cl.controlPlaneStack.Config.CloudFormation.RoleARN,
	)
}

func (cl Cluster) stackName() string {
	return cl.controlPlaneStack.Config.ClusterName
}

func (cl Cluster) tags() map[string]string {
	cptags := cl.controlPlaneStack.Config.StackTags
	if len(cptags) == 0 {
		cptags = make(map[string]string, 1)
	}
	cptags["kube-aws:version"] = model.VERSION
	return cptags
}

func (cl *Cluster) Apply(targets OperationTargets) error {
	cfSvc := cloudformation.New(cl.session)

	exists, err := cfnstack.StackExists(cl.context().ProvidedCFInterrogator, cl.controlPlaneStack.ClusterName)
	if err != nil {
		logger.Errorf("please check your AWS credentials/permissions")
		return fmt.Errorf("can't lookup AWS CloudFormation stacks: %s", err)
	}

	if exists {
		report, err := cl.update(cfSvc, targets)
		if err != nil {
			return fmt.Errorf("error updating cluster: %v", err)
		}
		if report != "" {
			logger.Infof("Update stack: %s\n", report)
		}
		return nil
	}
	return cl.create(cfSvc)
}

// remove with legacy up command
func (cl *Cluster) LegacyUpdate(targets OperationTargets) (string, error) {
	cfSvc := cloudformation.New(cl.session)
	return cl.update(cfSvc, targets)
}

func (cl *Cluster) update(cfSvc *cloudformation.CloudFormation, targets OperationTargets) (string, error) {

	assets, err := cl.generateAssets(cl.operationTargetsFromUserInput([]OperationTargets{targets}))
	if err != nil {
		return "", err
	}

	err = cl.uploadAssets(assets)
	if err != nil {
		return "", err
	}

	templateUrl, err := cl.extractRootStackTemplateURL(assets)
	if err != nil {
		return "", err
	}

	q := make(chan struct{}, 1)
	defer func() { q <- struct{}{} }()

	if cl.controlPlaneStack.Config.CloudWatchLogging.Enabled && cl.controlPlaneStack.Config.CloudWatchLogging.LocalStreaming.Enabled {
		go streamJournaldLogs(cl, q)
	}

	if cl.controlPlaneStack.Config.CloudFormationStreaming {
		go streamStackEvents(cl, cfSvc, q)
	}

	return cl.stackProvisioner().UpdateStackAtURLAndWait(cfSvc, templateUrl)
}

func (cl *Cluster) ValidateTemplates() error {
	_, err := cl.renderTemplateAsString()
	if err != nil {
		return fmt.Errorf("failed to validate template: %v", err)
	}
	if _, err := cl.networkStack.RenderStackTemplateAsString(); err != nil {
		return fmt.Errorf("failed to validate network template: %v", err)
	}
	if _, err := cl.etcdStack.RenderStackTemplateAsString(); err != nil {
		return fmt.Errorf("failed to validate etcd template: %v", err)
	}
	if _, err := cl.controlPlaneStack.RenderStackTemplateAsString(); err != nil {
		return fmt.Errorf("failed to validate control plane template: %v", err)
	}
	for i, p := range cl.nodePoolStacks {
		if _, err := p.RenderStackTemplateAsString(); err != nil {
			return fmt.Errorf("failed to validate node pool #%d template: %v", i, err)
		}
	}
	return nil
}

// ValidateStack validates all the CloudFormation stack templates already uploaded to S3
func (cl *Cluster) ValidateStack(opts ...OperationTargets) (string, error) {
	if err := cl.ensureNestedStacksLoaded(); err != nil {
		return "", err
	}

	reports := []string{}

	targets := cl.operationTargetsFromUserInput(opts)

	assets, err := cl.generateAssets(cl.operationTargetsFromUserInput([]OperationTargets{targets}))
	if err != nil {
		return "", err
	}

	// Send all the assets including stack templates and cloud-configs for all the stacks
	err = cl.uploadAssets(assets)
	if err != nil {
		return "", err
	}

	rootStackTemplateURL, err := cl.extractRootStackTemplateURL(assets)
	if err != nil {
		return "", err
	}

	r, err := cl.stackProvisioner().ValidateStackAtURL(rootStackTemplateURL)
	if err != nil {
		return "", err
	}

	reports = append(reports, r)

	ctx := cl.context()

	netReport, err := ctx.ValidateStack(cl.networkStack)
	if err != nil {
		return "", fmt.Errorf("failed to validate network: %v", err)
	}
	reports = append(reports, netReport)

	cpReport, err := ctx.ValidateStack(cl.controlPlaneStack)
	if err != nil {
		return "", fmt.Errorf("failed to validate control plane: %v", err)
	}
	reports = append(reports, cpReport)

	etcdReport, err := ctx.ValidateStack(cl.etcdStack)
	if err != nil {
		return "", fmt.Errorf("failed to validate etcd plane: %v", err)
	}
	reports = append(reports, etcdReport)

	for i, p := range cl.nodePoolStacks {
		npReport, err := ctx.ValidateStack(p)
		if err != nil {
			return "", fmt.Errorf("failed to validate node pool #%d: %v", i, err)
		}
		reports = append(reports, npReport)
	}

	return strings.Join(reports, "\n"), nil
}

func streamJournaldLogs(c *Cluster, q chan struct{}) error {
	logger.Infof("Streaming filtered Journald logs for log group '%s'...\nNOTE: Due to high initial entropy, '.service' failures may occur during the early stages of booting.\n", c.controlPlaneStack.ClusterName)
	cwlSvc := cloudwatchlogs.New(c.session)
	s := time.Now().Unix() * 1E3
	t := s
	in := cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:  &c.controlPlaneStack.ClusterName,
		FilterPattern: &c.controlPlaneStack.Config.CloudWatchLogging.LocalStreaming.Filter,
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
					if *event.Timestamp > ms[*event.Message]+c.controlPlaneStack.Config.CloudWatchLogging.LocalStreaming.IntervalSec() {
						ms[*event.Message] = *event.Timestamp
						res := api.SystemdMessageResponse{}
						json.Unmarshal([]byte(*event.Message), &res)
						s := int(((*event.Timestamp) - t) / 1E3)
						d := fmt.Sprintf("+%.2d:%.2d:%.2d", s/3600, (s/60)%60, s%60)
						logger.Infof("%s\t%s: \"%s\"\n", d, res.Hostname, res.Message)
					}
				}
			}
			in = cloudwatchlogs.FilterLogEventsInput{
				LogGroupName:  &c.controlPlaneStack.ClusterName,
				FilterPattern: &c.controlPlaneStack.Config.CloudWatchLogging.LocalStreaming.Filter,
				NextToken:     out.NextToken,
				StartTime:     &s}
		}
	}
}

// streamStackEvents streams all the events from the root, the control-plane, and worker node pool stacks using StreamEventsNested
func streamStackEvents(c *Cluster, cfSvc *cloudformation.CloudFormation, q chan struct{}) error {
	logger.Infof("Streaming CloudFormation events for the cluster '%s'...\n", c.controlPlaneStack.ClusterName)
	return c.stackProvisioner().StreamEventsNested(q, cfSvc, c.controlPlaneStack.ClusterName, c.controlPlaneStack.ClusterName, time.Now())
}
