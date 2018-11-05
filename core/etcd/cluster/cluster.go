package cluster

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/kubernetes-incubator/kube-aws/cfnstack"
	controlplanecluster "github.com/kubernetes-incubator/kube-aws/core/controlplane/cluster"
	controlplaneconfig "github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	"github.com/kubernetes-incubator/kube-aws/core/etcd/config"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/kubernetes-incubator/kube-aws/model"
	"github.com/kubernetes-incubator/kube-aws/naming"
	"github.com/kubernetes-incubator/kube-aws/plugin/clusterextension"
	"github.com/kubernetes-incubator/kube-aws/plugin/pluginmodel"
)

// VERSION set by build script
var VERSION = "UNKNOWN"

const STACK_TEMPLATE_FILENAME = "stack.json"

func newClusterRef(cfg *controlplaneconfig.Cluster, session *session.Session) *ClusterRef {
	return &ClusterRef{
		Cluster: cfg,
		session: session,
	}
}

type ClusterRef struct {
	*controlplaneconfig.Cluster
	session *session.Session
}

type Cluster struct {
	*ClusterRef
	*config.StackConfig
	assets cfnstack.Assets
}

// An EtcdConfigurationContext contains configuration settings/options mixed with existing state in a way that can be
// consumed by stack and cloud-config templates.
type EtcdConfigurationContext struct {
	*controlplaneconfig.Config
	model.EtcdExistingState
}

type ec2Service interface {
	CreateVolume(*ec2.CreateVolumeInput) (*ec2.Volume, error)
	DescribeVpcs(*ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error)
	DescribeSubnets(*ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error)
	DescribeKeyPairs(*ec2.DescribeKeyPairsInput) (*ec2.DescribeKeyPairsOutput, error)
}

func (c *ClusterRef) validateExistingVPCState(ec2Svc ec2Service) error {
	if !c.VPC.HasIdentifier() {
		//The VPC will be created. No existing state to validate
		return nil
	}

	// TODO kube-aws should de-reference the vpc id from the stack output and continue validating with it
	if c.VPC.IDFromStackOutput != "" {
		logger.Infof("kube-aws doesn't support validating the vpc referenced by the stack output `%s`. Skipped validation of existing vpc state. The cluster creation may fail afterwards if the VPC isn't configured properly.", c.VPC.IDFromStackOutput)
		return nil
	}

	vpcId := c.VPC.ID

	describeVpcsInput := ec2.DescribeVpcsInput{
		VpcIds: []*string{aws.String(vpcId)},
	}
	vpcOutput, err := ec2Svc.DescribeVpcs(&describeVpcsInput)
	if err != nil {
		return fmt.Errorf("error describing existing vpc: %v", err)
	}
	if len(vpcOutput.Vpcs) == 0 {
		return fmt.Errorf("could not find vpc %s in region %s", vpcId, c.Region)
	}
	if len(vpcOutput.Vpcs) > 1 {
		//Theoretically this should never happen. If it does, we probably want to know.
		return fmt.Errorf("found more than one vpc with id %s. this is NOT NORMAL", vpcId)
	}

	existingVPC := vpcOutput.Vpcs[0]

	if *existingVPC.CidrBlock != c.VPCCIDR {
		//If this is the case, our network config validation cannot be trusted and we must abort
		return fmt.Errorf(
			"configured vpcCidr (%s) does not match actual existing vpc cidr (%s)",
			c.VPCCIDR,
			*existingVPC.CidrBlock,
		)
	}

	describeSubnetsInput := ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{existingVPC.VpcId},
			},
		},
	}

	subnetOutput, err := ec2Svc.DescribeSubnets(&describeSubnetsInput)
	if err != nil {
		return fmt.Errorf("error describing subnets for vpc: %v", err)
	}

	subnetCIDRS := make([]string, len(subnetOutput.Subnets))
	for i, existingSubnet := range subnetOutput.Subnets {
		subnetCIDRS[i] = *existingSubnet.CidrBlock
	}

	if err := c.ValidateExistingVPC(*existingVPC.CidrBlock, subnetCIDRS); err != nil {
		return fmt.Errorf("error validating existing VPC: %v", err)
	}

	return nil
}

func NewCluster(cfgRef *controlplaneconfig.Cluster, opts controlplaneconfig.StackTemplateOptions, plugins []*pluginmodel.Plugin, session *session.Session) (*Cluster, error) {
	logger.Debugf("Called etcd.NewCluster")
	cfg := &controlplaneconfig.Cluster{}
	*cfg = *cfgRef

	// Import all the managed subnets from the network stack
	var err error
	cfg.Subnets, err = cfg.Subnets.ImportFromNetworkStackRetainingNames()
	if err != nil {
		return nil, fmt.Errorf("failed to import subnets from network stack: %v", err)
	}
	cfg.VPC = cfg.VPC.ImportFromNetworkStack()
	cfg.SetDefaults()

	clusterRef := newClusterRef(cfg, session)
	// TODO Do this in a cleaner way e.g. in config.go
	clusterRef.KubeResourcesAutosave.S3Path = model.NewS3Folders(cfg.DeploymentSettings.S3URI, clusterRef.ClusterName).ClusterBackups().Path()
	clusterRef.KubeAWSVersion = controlplanecluster.VERSION
	clusterRef.HostOS = cfgRef.HostOS

	cpStackConfig, err := clusterRef.StackConfig(cfgRef.EtcdStackName(), opts, session, plugins)
	if err != nil {
		return nil, err
	}
	// hack - mutate our controlplane generated stack config into our own specific etcd version
	etcdStackConfig := config.NewEtcdStackConfig(cpStackConfig)

	c := &Cluster{
		ClusterRef:  clusterRef,
		StackConfig: etcdStackConfig,
	}

	// Notes:
	// * `c.StackConfig.CustomSystemdUnits` results in an `ambiguous selector ` error
	// * `c.Etcd.CustomSystemdUnits = controllerUnits` and `c.ClusterRef.Etcd.CustomSystemdUnits = etcdUnits` results in modifying invisible/duplicate CustomSystemdSettings
	extras := clusterextension.NewExtrasFromPlugins(plugins, c.PluginConfigs)

	extraStack, err := extras.EtcdStack()
	if err != nil {
		return nil, fmt.Errorf("failed to load etcd stack extras from plugins: %v", err)
	}
	c.StackConfig.ExtraCfnResources = extraStack.Resources

	extraEtcd, err := extras.Etcd()
	if err != nil {
		return nil, fmt.Errorf("failed to load etcd node extras from plugins: %v", err)
	}
	c.StackConfig.Etcd.CustomSystemdUnits = append(c.StackConfig.Etcd.CustomSystemdUnits, extraEtcd.SystemdUnits...)
	c.StackConfig.Etcd.CustomFiles = append(c.StackConfig.Etcd.CustomFiles, extraEtcd.Files...)
	c.StackConfig.Etcd.IAMConfig.Policy.Statements = append(c.StackConfig.Etcd.IAMConfig.Policy.Statements, extraEtcd.IAMPolicyStatements...)

	// create the context that will be used to build the assets (combination of config + existing state)
	c.StackConfig.EtcdExistingState, err = c.inspectExistingState()
	if err != nil {
		return nil, fmt.Errorf("Could not inspect existing etcd state: %v", err)
	}
	ctx := EtcdConfigurationContext{
		Config:            c.StackConfig.Config,
		EtcdExistingState: c.StackConfig.EtcdExistingState,
	}

	c.assets, err = c.buildAssets(ctx)

	return c, err
}

func (c *Cluster) inspectExistingState() (model.EtcdExistingState, error) {
	var err error
	if c.ProvidedCFInterrogator == nil {
		c.ProvidedCFInterrogator = cloudformation.New(c.session)
	}
	if c.ProvidedEC2Interrogator == nil {
		c.ProvidedEC2Interrogator = ec2.New(c.session)
	}

	state := model.EtcdExistingState{}
	state.StackExists, err = cfnstack.NestedStackExists(c.ProvidedCFInterrogator, c.ClusterName, naming.FromStackToCfnResource(c.Etcd.LogicalName()))
	if err != nil {
		return state, fmt.Errorf("failed to check for existence of etcd cloud-formation stack: %v", err)
	}
	// when the Etcd stack does not exist but we have existing etcd instances then we need to enable the
	// etcd migration units.
	if !state.StackExists {
		if state.EtcdMigrationExistingEndpoints, err = c.lookupExistingEtcdEndpoints(); err != nil {
			return state, fmt.Errorf("failed to lookup existing etcd endpoints: %v", err)
		}
		if state.EtcdMigrationExistingEndpoints != "" {
			state.EtcdMigrationEnabled = true
		}
	}
	return state, nil
}

func (c *Cluster) Assets() cfnstack.Assets {
	return c.assets
}

// NestedStackName returns a sanitized name of this etcd which is usable as a valid cloudformation nested stack name
func (c Cluster) NestedStackName() string {
	// Convert stack name into something valid as a cfn resource name or
	// we'll end up with cfn errors like "Template format error: Resource name test5-etcd is non alphanumeric"
	return naming.FromStackToCfnResource(c.StackName)
}

func (c *Cluster) buildAssets(ctx EtcdConfigurationContext) (cfnstack.Assets, error) {
	logger.Debugf("Called etcd.buildAssets")
	logger.Debugf("Context is: %+v", ctx)
	var err error
	assets := cfnstack.NewAssetsBuilder(c.StackName, c.StackConfig.ClusterExportedStacksS3URI(), c.StackConfig.Region)

	if c.StackConfig.UserDataEtcd, err = model.NewUserData(c.StackTemplateOptions.EtcdTmplFile, ctx); err != nil {
		return nil, fmt.Errorf("failed to render etcd cloud config: %v", err)
	}

	if err = assets.AddUserDataPart(c.UserDataEtcd, model.USERDATA_S3, "userdata-etcd"); err != nil {
		return nil, fmt.Errorf("failed to render etcd cloud config: %v", err)
	}

	stackTemplate, err := c.RenderStackTemplateAsString()
	if err != nil {
		return nil, fmt.Errorf("Error while rendering template: %v", err)
	}

	logger.Debugf("Calling assets.Add on %s", STACK_TEMPLATE_FILENAME)
	assets.Add(STACK_TEMPLATE_FILENAME, stackTemplate)

	logger.Debugf("Calling assets.Build for etcd...")
	return assets.Build(), nil
}

func (c *Cluster) TemplateURL() (string, error) {
	assets := c.Assets()
	asset, err := assets.FindAssetByStackAndFileName(c.StackName, STACK_TEMPLATE_FILENAME)
	if err != nil {
		return "", fmt.Errorf("failed to get template URL: %v", err)
	}
	return asset.URL()
}

// ValidateStack validates the CloudFormation stack for this control plane already uploaded to S3
func (c *Cluster) ValidateStack() (string, error) {
	templateURL, err := c.TemplateURL()
	if err != nil {
		return "", fmt.Errorf("failed to get template url : %v", err)
	}
	return c.stackProvisioner().ValidateStackAtURL(templateURL)
}

func (c *Cluster) stackProvisioner() *cfnstack.Provisioner {
	stackPolicyBody := `{
  "Statement" : [
    {
       "Effect" : "Allow",
       "Principal" : "*",
       "Action" : "Update:*",
       "Resource" : "*"
     }
  ]
}
`
	return cfnstack.NewProvisioner(
		c.StackName,
		c.StackTags,
		c.ClusterExportedStacksS3URI(),
		c.Region,
		stackPolicyBody,
		c.session,
		c.CloudFormation.RoleARN,
	)
}

func (c *Cluster) Validate() error {
	ec2Svc := ec2.New(c.session)

	if err := c.validateExistingVPCState(ec2Svc); err != nil {
		return err
	}

	return nil
}

func (c *Cluster) String() string {
	return fmt.Sprintf("{Config:%+v}", *c.StackConfig.Config)
}

func (c *ClusterRef) Destroy() error {
	return cfnstack.NewDestroyer(c.EtcdStackName(), c.session, c.CloudFormation.RoleARN).Destroy()
}

// lookupExistingEtcdEndpoints supports the migration from embedded etcd servers to their own stack
// by looking up the existing etcd servers for a specific cluster and constructing and etcd endpoints
// list as used by tools such as etcdctl and the etcdadm script.
func (c Cluster) lookupExistingEtcdEndpoints() (string, error) {
	clusterTag := fmt.Sprintf("tag:kubernetes.io/cluster/%s", c.ClusterName)
	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:kube-aws:role"),
				Values: []*string{aws.String("etcd")},
			},
			{
				Name:   aws.String(clusterTag),
				Values: []*string{aws.String("owned")},
			},
			{
				Name:   aws.String("instance-state-name"),
				Values: []*string{aws.String("running"), aws.String("pending")},
			},
		},
	}
	logger.Debugf("Calling AWS EC2 DescribeInstances ->")
	resp, err := c.ProvidedEC2Interrogator.DescribeInstances(params)
	if err != nil {
		return "", fmt.Errorf("can't lookup ec2 instances: %v", err)
	}
	if resp == nil {
		return "", nil
	}

	logger.Debugf("<- received %d instances from AWS", len(resp.Reservations))
	if len(resp.Reservations) == 0 {
		return "", nil
	}
	// construct comma separated endpoints string
	endpoints := []string{}
	for _, res := range resp.Reservations {
		for _, inst := range res.Instances {
			endpoints = append(endpoints, fmt.Sprintf("https://%s:2379", *inst.PrivateDnsName))
		}
	}
	result := strings.Join(endpoints, ",")
	logger.Debugf("Existing etcd endpoints found: %s", result)
	return result, nil
}
