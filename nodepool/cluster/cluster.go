package cluster

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/coreos/kube-aws/cfnstack"
	"github.com/coreos/kube-aws/nodepool/config"
	"text/tabwriter"
)

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
	stackTemplate, err := c.RenderStackTemplateAsString()
	if err != nil {
		return "", fmt.Errorf("failed to validate template : %v", err)
	}
	return c.stackProvisioner().Validate(stackTemplate)
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
