package cluster

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/route53"

	"github.com/kubernetes-incubator/kube-aws/cfnstack"
	"github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	"github.com/kubernetes-incubator/kube-aws/model"
	"github.com/kubernetes-incubator/kube-aws/plugin/pluginapi"
	"github.com/kubernetes-incubator/kube-aws/plugin/plugincontents"
	"github.com/kubernetes-incubator/kube-aws/plugin/pluginvalue"
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
		fmt.Printf("kube-aws doesn't support validating the vpc referenced by the stack output `%s`. Skipped validation of exsiting vpc state. The cluster creation may fail afterwards if the VPC isn't configured properly.", c.VPC.IDFromStackOutput)
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

func NewCluster(cfg *config.Cluster, opts config.StackTemplateOptions, plugins []*pluginapi.Plugin, awsDebug bool) (*Cluster, error) {
	clusterRef := NewClusterRef(cfg, awsDebug)
	// TODO Do this in a cleaner way e.g. in config.go
	clusterRef.KubeResourcesAutosave.S3Path = model.NewS3Folders(opts.S3URI, clusterRef.ClusterName).ClusterBackups().Path()

	stackConfig, err := clusterRef.StackConfig(opts, plugins)
	if err != nil {
		return nil, err
	}

	c := &Cluster{
		ClusterRef:  clusterRef,
		StackConfig: stackConfig,
	}

	// Notes:
	// * `c.StackConfig.CustomSystemdUnits` results in an `ambiguous selector ` error
	// * `c.Controller.CustomSystemdUnits = controllerUnits` and `c.ClusterRef.Controller.CustomSystemdUnits = controllerUnits` results in modifying invisible/duplicate CustomSystemdSettings
	for _, p := range plugins {
		if enabled, pc := p.EnabledIn(c.PluginConfigs); enabled {
			values := p.Spec.Values.Merge(pc.Values)

			load := plugincontents.LoaderFor(p)
			render := plugincontents.TemplateRendererFor(p, values)

			{
				m, err := render.MapFromContents(p.Spec.CloudFormation.Stacks.ControlPlane.Resources.Append.Contents)
				if err != nil {
					return nil, fmt.Errorf("failed to load additioanl resources for control-plane stack: %v", err)
				}
				for k, v := range m {
					c.StackConfig.AdditionalCfnResources[k] = v
				}
			}

			{
				render := pluginvalue.TemplateRendererFor(p, values)
				for _, f := range p.Spec.Kubernetes.APIServer.Flags {
					v, err := render.StringFrom(f.Value)
					if err != nil {
						return nil, fmt.Errorf("failed to load apisersver flags: %v", err)
					}
					newFlag := pluginapi.APIServerFlag{
						Name:  f.Name,
						Value: v,
					}
					c.StackConfig.Config.APIServerFlags = append(c.StackConfig.APIServerFlags, newFlag)
				}
			}

			{
				c.StackConfig.APIServerVolumes = p.Spec.Kubernetes.APIServer.Volumes
			}

			for _, d := range p.Spec.Node.Roles.Controller.Systemd.Units {
				u := model.CustomSystemdUnit{
					Name:    d.Name,
					Command: "start",
					Content: d.Contents.Inline,
					Enable:  true,
					Runtime: false,
				}
				c.StackConfig.Controller.CustomSystemdUnits = append(c.StackConfig.Controller.CustomSystemdUnits, u)
			}
			for _, d := range p.Spec.Node.Roles.Controller.Storage.Files {
				s, err := load.StringFrom(d.Contents)
				if err != nil {
					return nil, fmt.Errorf("failed to load plugin controller file contents: %v", err)
				}
				f := model.CustomFile{
					Path:        d.Path,
					Permissions: d.Permissions,
					Content:     s,
				}
				c.StackConfig.Controller.CustomFiles = append(c.StackConfig.Controller.CustomFiles, f)
			}
			c.StackConfig.Controller.IAMConfig.Policy.Statements = append(c.StackConfig.Controller.IAMConfig.Policy.Statements, p.Spec.Node.Roles.Controller.IAM.Policy.Statements...)

			for k, v := range p.Spec.Node.Roles.Controller.Kubelet.NodeLabels {
				c.StackConfig.Controller.NodeLabels[k] = v
			}

			for _, d := range p.Spec.Node.Roles.Etcd.Systemd.Units {
				u := model.CustomSystemdUnit{
					Name:    d.Name,
					Command: "start",
					Content: d.Contents.Inline,
					Enable:  true,
					Runtime: false,
				}
				c.StackConfig.Etcd.CustomSystemdUnits = append(c.StackConfig.Etcd.CustomSystemdUnits, u)
			}
			for _, d := range p.Spec.Node.Roles.Etcd.Storage.Files {
				s, err := load.StringFrom(d.Contents)
				if err != nil {
					return nil, fmt.Errorf("failed to load plugin etcd file contents: %v", err)
				}
				f := model.CustomFile{
					Path:        d.Path,
					Permissions: d.Permissions,
					Content:     s,
				}
				c.StackConfig.Etcd.CustomFiles = append(c.StackConfig.Etcd.CustomFiles, f)
			}
			c.StackConfig.Etcd.IAMConfig.Policy.Statements = append(c.StackConfig.Etcd.IAMConfig.Policy.Statements, p.Spec.Node.Roles.Etcd.IAM.Policy.Statements...)
		}
	}

	c.assets, err = c.buildAssets()

	return c, err
}

func (c *Cluster) Assets() cfnstack.Assets {
	return c.assets
}

func (c *Cluster) buildAssets() (cfnstack.Assets, error) {
	var err error
	assets := cfnstack.NewAssetsBuilder(c.StackName(), c.StackConfig.ClusterExportedStacksS3URI(), c.StackConfig.Region)

	if c.StackConfig.UserDataController, err = model.NewUserData(c.StackTemplateOptions.ControllerTmplFile, c.StackConfig.Config); err != nil {
		return nil, fmt.Errorf("failed to render controller cloud config: %v", err)
	}

	if c.StackConfig.UserDataEtcd, err = model.NewUserData(c.StackTemplateOptions.EtcdTmplFile, c.StackConfig.Config); err != nil {
		return nil, fmt.Errorf("failed to render etcd cloud config: %v", err)
	}

	if err = assets.AddUserDataPart(c.UserDataController, model.USERDATA_S3, "userdata-controller"); err != nil {
		return nil, fmt.Errorf("failed to render controller cloud config: %v", err)
	}

	if err = assets.AddUserDataPart(c.UserDataEtcd, model.USERDATA_S3, "userdata-etcd"); err != nil {
		return nil, fmt.Errorf("failed to render etcd cloud config: %v", err)
	}

	stackTemplate, err := c.RenderStackTemplateAsString()
	if err != nil {
		return nil, fmt.Errorf("Error while rendering template: %v", err)
	}

	assets.Add(STACK_TEMPLATE_FILENAME, stackTemplate)

	return assets.Build(), nil
}

func (c *Cluster) TemplateURL() (string, error) {
	assets := c.Assets()
	asset, err := assets.FindAssetByStackAndFileName(c.StackName(), STACK_TEMPLATE_FILENAME)
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
		c.StackName(),
		c.StackTags,
		c.ClusterExportedStacksS3URI(),
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

	if err := c.validateDNSConfig(route53.New(c.session)); err != nil {
		return err
	}

	return nil
}

func (c *Cluster) String() string {
	return fmt.Sprintf("{Config:%+v}", *c.StackConfig.Config)
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
		Iops:             aws.Int64(int64(c.Controller.RootVolume.IOPS)),
		Size:             aws.Int64(int64(c.Controller.RootVolume.Size)),
		VolumeType:       aws.String(c.Controller.RootVolume.Type),
	}

	if _, err := ec2Svc.CreateVolume(controllerRootVolume); err != nil {
		if operr, ok := err.(awserr.Error); ok && operr.Code() != "DryRunOperation" {
			return fmt.Errorf("create volume dry-run request failed: %v", err)
		}
	}

	return nil
}
