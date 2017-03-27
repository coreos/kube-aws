package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/route53"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/kubernetes-incubator/kube-aws/cfnstack"
	"github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
)

// VERSION set by build script
var VERSION = "UNKNOWN"

const STACK_TEMPLATE_FILENAME = "stack.json"

func NewClusterRef(cfg *config.Cluster, awsDebug bool) *ClusterRef {
	awsConfig := aws.NewConfig().
		WithRegion(cfg.Region.String()).
		WithCredentialsChainVerboseErrors(true)

	if awsDebug {
		awsConfig = awsConfig.WithLogLevel(aws.LogDebug)
	}

	return &ClusterRef{
		Cluster: cfg,
		session: session.New(awsConfig),
	}
}

type ClusterRef struct {
	*config.Cluster
	session *session.Session
}

type Cluster struct {
	*ClusterRef
	*config.CompressedStackConfig
}

type ec2Service interface {
	CreateVolume(*ec2.CreateVolumeInput) (*ec2.Volume, error)
	DescribeVpcs(*ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error)
	DescribeSubnets(*ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error)
	DescribeKeyPairs(*ec2.DescribeKeyPairsInput) (*ec2.DescribeKeyPairsOutput, error)
}

func (c *ClusterRef) validateExistingVPCState(ec2Svc ec2Service) error {
	if c.VPCID == "" {
		//The VPC will be created. No existing state to validate
		return nil
	}

	describeVpcsInput := ec2.DescribeVpcsInput{
		VpcIds: []*string{aws.String(c.VPCID)},
	}
	vpcOutput, err := ec2Svc.DescribeVpcs(&describeVpcsInput)
	if err != nil {
		return fmt.Errorf("error describing existing vpc: %v", err)
	}
	if len(vpcOutput.Vpcs) == 0 {
		return fmt.Errorf("could not find vpc %s in region %s", c.VPCID, c.Region)
	}
	if len(vpcOutput.Vpcs) > 1 {
		//Theoretically this should never happen. If it does, we probably want to know.
		return fmt.Errorf("found more than one vpc with id %s. this is NOT NORMAL", c.VPCID)
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

func NewCluster(cfg *config.Cluster, opts config.StackTemplateOptions, awsDebug bool) (*Cluster, error) {
	cluster := NewClusterRef(cfg, awsDebug)
	stackConfig, err := cluster.StackConfig(opts)
	if err != nil {
		return nil, err
	}
	compressed, err := stackConfig.Compress()
	if err != nil {
		return nil, err
	}
	return &Cluster{
		ClusterRef:            cluster,
		CompressedStackConfig: compressed,
	}, nil
}

func (c *Cluster) Assets() (cfnstack.Assets, error) {
	stackTemplate, err := c.RenderTemplateAsString()
	if err != nil {
		return nil, fmt.Errorf("Error while rendering template : %v", err)
	}

	return cfnstack.NewAssetsBuilder(c.StackName(), c.StackConfig.S3URI, c.StackConfig.Region).
		Add(c.UserDataControllerFileName(), c.UserDataController).
		Add(c.UserDataEtcdFileName(), c.UserDataEtcd).
		Add(STACK_TEMPLATE_FILENAME, stackTemplate).
		Build(), nil
}

func (c *Cluster) TemplateURL() (string, error) {
	assets, err := c.Assets()
	if err != nil {
		return "", err
	}
	asset, err := assets.FindAssetByStackAndFileName(c.StackName(), STACK_TEMPLATE_FILENAME)
	if err != nil {
		return "", fmt.Errorf("failed to get template URL: %v", err)
	}

	return asset.URL(), nil
}

// ValidateStack validates the CloudFormation stack for this control plane already uploaded to S3
func (c *Cluster) ValidateStack() (string, error) {
	if err := c.ValidateUserData(); err != nil {
		return "", fmt.Errorf("failed to validate userdata : %v", err)
	}

	templateURL, err := c.TemplateURL()
	if err != nil {
		return "", fmt.Errorf("failed to get template url : %v", err)
	}
	return c.stackProvisioner().ValidateStackAtURL(templateURL)
}

func (c *Cluster) RenderTemplateAsString() (string, error) {
	data, err := c.RenderStackTemplateAsString()
	if err != nil {
		return "", fmt.Errorf("Error while rendering stack template : %v", err)
	}
	return data, nil
}

func (c *Cluster) stackProvisioner() *cfnstack.Provisioner {
	stackPolicyBody := `{
  "Statement" : [
    {
      "Effect" : "Deny",
      "Action" : "Update:*",
      "Principal" : "*",
      "Resource" : "LogicalResourceId/InstanceEtcd*"
    },
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
		c.StackName(),
		c.StackTags,
		c.S3URI,
		c.Region,
		stackPolicyBody,
		c.session)
}

func (c *Cluster) Validate() error {
	ec2Svc := ec2.New(c.session)
	if c.KeyName != "" {
		if err := c.validateKeyPair(ec2Svc); err != nil {
			return err
		}
	}

	if err := c.validateExistingVPCState(ec2Svc); err != nil {
		return err
	}

	if err := c.validateControllerRootVolume(ec2Svc); err != nil {
		return err
	}

	return nil
}

func (c *Cluster) Create() error {
	r53Svc := route53.New(c.session)
	if err := c.validateDNSConfig(r53Svc); err != nil {
		return err
	}

	if err := c.Validate(); err != nil {
		return err
	}

	cfSvc := cloudformation.New(c.session)
	s3Svc := s3.New(c.session)

	stackTemplate, err := c.RenderTemplateAsString()
	if err != nil {
		return fmt.Errorf("Error while rendering template : %v", err)
	}

	cloudConfigs := map[string]string{
		"userdata-controller": c.UserDataController,
		"userdata-worker":     c.UserDataWorker,
	}

	return c.stackProvisioner().CreateStackAndWait(cfSvc, s3Svc, stackTemplate, cloudConfigs)
}

/*
Makes sure that etcd resource definitions are not upgrades by cloudformation stack update.
Fetches resource defintions from existing stack and splices them into the updated resource defintions.

TODO(chom): etcd controller + dynamic cluster management will obviate need for this function
*/
type cfStackResources struct {
	Resources map[string]map[string]interface{} `json:"Resources"`
	Mappings  map[string]interface{}            `json:"Mappings"`
}

func (c *ClusterRef) lockEtcdResources(cfSvc *cloudformation.CloudFormation, stackBody string) (string, error) {

	//Unmarshal incoming stack resource defintions
	var newStack cfStackResources

	if err := json.Unmarshal([]byte(stackBody), &newStack); err != nil {
		return "", fmt.Errorf("error unmarshaling new stack json: %v", err)
	}

	instanceEtcdExpr := regexp.MustCompile("^InstanceEtcd[0-9]+$")
	//Remove all etcdInstance resource defintions from incoming stack
	for name, _ := range newStack.Resources {
		if instanceEtcdExpr.Match([]byte(name)) {
			delete(newStack.Resources, name)
		}
	}

	//Fetch and unmarshal existing stack resource defintions
	res, err := cfSvc.GetTemplate(&cloudformation.GetTemplateInput{
		StackName: aws.String(c.StackName()),
	})
	if err != nil {
		return "", fmt.Errorf("error getting stack template: %v", err)
	}
	var existingStack cfStackResources
	if err := json.Unmarshal([]byte(*res.TemplateBody), &existingStack); err != nil {
		return "", fmt.Errorf("error unmarshaling existing stack json: %v", err)
	}

	//splice in existing resource defintions for etcd into new stack
	for name, definition := range existingStack.Resources {
		if instanceEtcdExpr.Match([]byte(name)) {
			newStack.Resources[name] = definition
		}
	}
	newStack.Mappings["EtcdInstanceParams"] = existingStack.Mappings["EtcdInstanceParams"]

	var outgoingStack map[string]interface{}
	if err := json.Unmarshal([]byte(stackBody), &outgoingStack); err != nil {
		return "", fmt.Errorf("error unmarshaling outgoing stack json: %v", err)
	}
	outgoingStack["Resources"] = newStack.Resources
	outgoingStack["Mappings"] = newStack.Mappings

	// ship off new stack to cloudformation api for an update
	out, err := json.Marshal(&outgoingStack)
	if err != nil {
		return "", fmt.Errorf("error marshaling stack json: %v", err)
	}

	var buf bytes.Buffer
	if err := json.Compact(&buf, out); err != nil {
		return "", fmt.Errorf("error compacting stack json: %v", err)
	}

	return buf.String(), nil
}

func (c *Cluster) String() string {
	return fmt.Sprintf("{Config:%+v}", *c.CompressedStackConfig.Config)
}

func (c *Cluster) Update() (string, error) {
	cfSvc := cloudformation.New(c.session)
	s3Svc := s3.New(c.session)

	var err error

	var stackTemplate string
	if stackTemplate, err = c.RenderTemplateAsString(); err != nil {
		return "", fmt.Errorf("Error while rendering template : %v", err)
	}

	var stackBody string
	if stackBody, err = c.lockEtcdResources(cfSvc, stackTemplate); err != nil {
		return "", err
	}

	cloudConfigs := map[string]string{
		"userdata-controller": c.UserDataController,
		"userdata-worker":     c.UserDataWorker,
		"userdata-etcd":       c.UserDataEtcd,
	}
	updateOutput, err := c.stackProvisioner().UpdateStackAndWait(cfSvc, s3Svc, stackBody, cloudConfigs)

	return updateOutput, err
}

func (c *ClusterRef) Destroy() error {
	return cfnstack.NewDestroyer(c.StackName(), c.session).Destroy()
}

func (c *ClusterRef) validateKeyPair(ec2Svc ec2Service) error {
	_, err := ec2Svc.DescribeKeyPairs(&ec2.DescribeKeyPairsInput{
		KeyNames: []*string{aws.String(c.KeyName)},
	})

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == "InvalidKeyPair.NotFound" {
				return fmt.Errorf("Key %s does not exist.", c.KeyName)
			}
		}
		return err
	}
	return nil
}

type r53Service interface {
	ListHostedZonesByName(*route53.ListHostedZonesByNameInput) (*route53.ListHostedZonesByNameOutput, error)
	ListResourceRecordSets(*route53.ListResourceRecordSetsInput) (*route53.ListResourceRecordSetsOutput, error)
	GetHostedZone(*route53.GetHostedZoneInput) (*route53.GetHostedZoneOutput, error)
}

func (c *ClusterRef) validateDNSConfig(r53 r53Service) error {
	if !c.CreateRecordSet {
		return nil
	}

	hzOut, err := r53.GetHostedZone(&route53.GetHostedZoneInput{Id: aws.String(c.HostedZoneID)})
	if err != nil {
		return fmt.Errorf("error getting hosted zone %s: %v", c.HostedZoneID, err)
	}

	if !isSubdomain(c.ExternalDNSName, aws.StringValue(hzOut.HostedZone.Name)) {
		return fmt.Errorf("externalDNSName %s is not a sub-domain of hosted-zone %s", c.ExternalDNSName, aws.StringValue(hzOut.HostedZone.Name))
	}

	recordSetsResp, err := r53.ListResourceRecordSets(
		&route53.ListResourceRecordSetsInput{
			HostedZoneId: hzOut.HostedZone.Id,
		},
	)
	if err != nil {
		return fmt.Errorf("error listing record sets for hosted zone id = %s: %v", c.HostedZoneID, err)
	}

	if len(recordSetsResp.ResourceRecordSets) > 0 {
		for _, recordSet := range recordSetsResp.ResourceRecordSets {
			if *recordSet.Name == config.WithTrailingDot(c.ExternalDNSName) {
				return fmt.Errorf(
					"RecordSet for \"%s\" already exists in Hosted Zone \"%s.\"",
					c.ExternalDNSName,
					c.HostedZoneID,
				)
			}
		}
	}

	return nil
}

func isSubdomain(sub, parent string) bool {
	sub, parent = config.WithTrailingDot(sub), config.WithTrailingDot(parent)
	subParts, parentParts := strings.Split(sub, "."), strings.Split(parent, ".")

	if len(parentParts) > len(subParts) {
		return false
	}

	subSuffixes := subParts[len(subParts)-len(parentParts):]

	if len(subSuffixes) != len(parentParts) {
		return false
	}
	for i := range subSuffixes {
		if subSuffixes[i] != parentParts[i] {
			return false
		}
	}
	return true
}

func (c *ClusterRef) validateControllerRootVolume(ec2Svc ec2Service) error {

	//Send a dry-run request to validate the controller root volume parameters
	controllerRootVolume := &ec2.CreateVolumeInput{
		DryRun:           aws.Bool(true),
		AvailabilityZone: aws.String(c.AvailabilityZones()[0]),
		Iops:             aws.Int64(int64(c.ControllerRootVolumeIOPS)),
		Size:             aws.Int64(int64(c.ControllerRootVolumeSize)),
		VolumeType:       aws.String(c.ControllerRootVolumeType),
	}

	if _, err := ec2Svc.CreateVolume(controllerRootVolume); err != nil {
		if operr, ok := err.(awserr.Error); ok && operr.Code() != "DryRunOperation" {
			return fmt.Errorf("create volume dry-run request failed: %v", err)
		}
	}

	return nil
}
