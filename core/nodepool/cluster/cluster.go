package cluster

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/coreos/kube-aws/cfnstack"
	"github.com/coreos/kube-aws/core/nodepool/config"
	"text/tabwriter"
)

const STACK_TEMPLATE_FILENAME = "stack.json"

type ClusterRef struct {
	config.ProvidedConfig
	session *session.Session
}

type Cluster struct {
	*ClusterRef
	*config.CompressedStackConfig
}

type Info struct {
	Name string
}

type ec2DescribeKeyPairsService interface {
	DescribeKeyPairs(*ec2.DescribeKeyPairsInput) (*ec2.DescribeKeyPairsOutput, error)
}

type ec2CreateVolumeService interface {
	CreateVolume(*ec2.CreateVolumeInput) (*ec2.Volume, error)
}

func (c *Info) String() string {
	buf := new(bytes.Buffer)
	w := new(tabwriter.Writer)
	w.Init(buf, 0, 8, 0, '\t', 0)

	fmt.Fprintf(w, "Cluster Name:\t%s\n", c.Name)

	w.Flush()
	return buf.String()
}

func NewClusterRef(cfg *config.ProvidedConfig, awsDebug bool) *ClusterRef {
	awsConfig := aws.NewConfig().
		WithRegion(cfg.Region).
		WithCredentialsChainVerboseErrors(true)

	if awsDebug {
		awsConfig = awsConfig.WithLogLevel(aws.LogDebug)
	}

	return &ClusterRef{
		ProvidedConfig: *cfg,
		session:        session.New(awsConfig),
	}
}

func NewCluster(provided *config.ProvidedConfig, opts config.StackTemplateOptions, awsDebug bool) (*Cluster, error) {
	computed, err := provided.Config()
	if err != nil {
		return nil, err
	}
	stackConfig, err := computed.StackConfig(opts)
	if err != nil {
		return nil, err
	}
	compressed, err := stackConfig.Compress()
	if err != nil {
		return nil, err
	}
	ref := NewClusterRef(provided, awsDebug)
	return &Cluster{
		CompressedStackConfig: compressed,
		ClusterRef:            ref,
	}, nil
}

func (c *Cluster) Assets() (cfnstack.Assets, error) {
	stackTemplate, err := c.RenderStackTemplateAsString()
	if err != nil {
		return nil, fmt.Errorf("Error while rendering template : %v", err)
	}

	return cfnstack.NewAssetsBuilder(c.StackName(), c.StackConfig.S3URI).
		Add("userdata-worker", c.UserDataWorker).
		Add(STACK_TEMPLATE_FILENAME, stackTemplate).
		Build(), nil
}

func (c *Cluster) TemplateURL() (string, error) {
	assets, err := c.Assets()
	if err != nil {
		return "", fmt.Errorf("failed to get template url: %v", err)
	}
	asset, err := assets.FindAssetByStackAndFileName(c.StackName(), STACK_TEMPLATE_FILENAME)
	if err != nil {
		return "", fmt.Errorf("failed to get template url: %v", err)
	}
	return asset.URL(), nil
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
}`

	return cfnstack.NewProvisioner(c.StackName(), c.WorkerDeploymentSettings().StackTags(), c.S3URI, stackPolicyBody, c.session())
}

func (c *Cluster) session() *session.Session {
	return c.ClusterRef.session
}

func (c *Cluster) Create() error {
	cfSvc := cloudformation.New(c.session())
	s3Svc := s3.New(c.session())
	stackTemplate, err := c.RenderStackTemplateAsString()
	if err != nil {
		return err
	}

	cloudConfigs := map[string]string{
		"userdata-worker": c.UserDataWorker,
	}

	return c.stackProvisioner().CreateStackAndWait(cfSvc, s3Svc, stackTemplate, cloudConfigs)
}

func (c *Cluster) Update() (string, error) {
	cfSvc := cloudformation.New(c.session())
	s3Svc := s3.New(c.session())
	stackTemplate, err := c.RenderStackTemplateAsString()
	if err != nil {
		return "", err
	}

	cloudConfigs := map[string]string{
		"userdata-worker": c.UserDataWorker,
	}

	updateOutput, err := c.stackProvisioner().UpdateStackAndWait(cfSvc, s3Svc, stackTemplate, cloudConfigs)

	return updateOutput, err
}

func (c *Cluster) ValidateStack() (string, error) {
	if err := c.ValidateUserData(); err != nil {
		return "", fmt.Errorf("failed to validate userdata : %v", err)
	}

	ec2Svc := ec2.New(c.session())
	if err := c.validateWorkerRootVolume(ec2Svc); err != nil {
		return "", err
	}
	if c.KeyName != "" {
		if err := c.validateKeyPair(ec2Svc); err != nil {
			return "", err
		}
	}

	stackTemplate, err := c.RenderStackTemplateAsString()
	if err != nil {
		return "", fmt.Errorf("failed to validate template : %v", err)
	}
	return c.stackProvisioner().Validate(stackTemplate)
}

func (c *ClusterRef) validateKeyPair(ec2Svc ec2DescribeKeyPairsService) error {
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

func (c *ClusterRef) validateWorkerRootVolume(ec2Svc ec2CreateVolumeService) error {

	//Send a dry-run request to validate the worker root volume parameters
	workerRootVolume := &ec2.CreateVolumeInput{
		DryRun:           aws.Bool(true),
		AvailabilityZone: aws.String(c.Subnets[0].AvailabilityZone),
		Iops:             aws.Int64(int64(c.RootVolumeIOPS)),
		Size:             aws.Int64(int64(c.RootVolumeSize)),
		VolumeType:       aws.String(c.RootVolumeType),
	}

	if _, err := ec2Svc.CreateVolume(workerRootVolume); err != nil {
		operr, ok := err.(awserr.Error)

		if !ok || (ok && operr.Code() != "DryRunOperation") {
			return fmt.Errorf("create volume dry-run request failed: %v", err)
		}
	}

	return nil
}

func (c *ClusterRef) Info() (*Info, error) {
	var info Info
	{
		info.Name = c.NodePoolName
	}
	return &info, nil
}

func (c *ClusterRef) Destroy() error {
	return cfnstack.NewDestroyer(c.StackName(), c.session).Destroy()
}
