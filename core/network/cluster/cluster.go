package cluster

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/kubernetes-incubator/kube-aws/cfnstack"
	"github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/kubernetes-incubator/kube-aws/model"
	"github.com/kubernetes-incubator/kube-aws/naming"
	"github.com/kubernetes-incubator/kube-aws/plugin/pluginmodel"
)

// VERSION set by build script
var VERSION = "UNKNOWN"

const STACK_TEMPLATE_FILENAME = "stack.json"

func newClusterRef(cfg *config.Cluster, session *session.Session) *ClusterRef {
	return &ClusterRef{
		Cluster: cfg,
		session: session,
	}
}

type ClusterRef struct {
	*config.Cluster
	session *session.Session
}

type Cluster struct {
	*ClusterRef
	*config.StackConfig
	assets cfnstack.Assets
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

func NewCluster(cfgRef *config.Cluster, opts config.StackTemplateOptions, plugins []*pluginmodel.Plugin, session *session.Session) (*Cluster, error) {
	cfg := &config.Cluster{}
	*cfg = *cfgRef

	clusterRef := newClusterRef(cfg, session)
	// TODO Do this in a cleaner way e.g. in config.go
	clusterRef.KubeResourcesAutosave.S3Path = model.NewS3Folders(cfg.DeploymentSettings.S3URI, clusterRef.ClusterName).ClusterBackups().Path()

	stackConfig, err := clusterRef.StackConfig("network", opts, session, plugins)
	if err != nil {
		return nil, err
	}

	c := &Cluster{
		ClusterRef:  clusterRef,
		StackConfig: stackConfig,
	}

	c.assets, err = c.buildAssets()

	return c, err
}

func (c *Cluster) Assets() cfnstack.Assets {
	return c.assets
}

// NestedStackName returns a sanitized name of this control-plane which is usable as a valid cloudformation nested stack name
func (c Cluster) NestedStackName() string {
	// Convert stack name into something valid as a cfn resource name or
	// we'll end up with cfn errors like "Template format error: Resource name test5-controlplane is non alphanumeric"
	return naming.FromStackToCfnResource(c.StackName)
}

func (c *Cluster) buildAssets() (cfnstack.Assets, error) {
	var err error
	assets := cfnstack.NewAssetsBuilder(c.StackName, c.StackConfig.ClusterExportedStacksS3URI(), c.StackConfig.Region)

	stackTemplate, err := c.RenderStackTemplateAsString()
	if err != nil {
		return nil, fmt.Errorf("Error while rendering template: %v", err)
	}

	assets.Add(STACK_TEMPLATE_FILENAME, stackTemplate)

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
	return cfnstack.NewDestroyer("network", c.session, c.CloudFormation.RoleARN).Destroy()
}
