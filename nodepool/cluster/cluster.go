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

type Cluster struct {
	config.StackConfig
	session *session.Session
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

func New(cfg *config.StackConfig, awsDebug bool) *Cluster {
	awsConfig := aws.NewConfig().
		WithRegion(cfg.Region).
		WithCredentialsChainVerboseErrors(true)

	if awsDebug {
		awsConfig = awsConfig.WithLogLevel(aws.LogDebug)
	}

	return &Cluster{
		StackConfig: *cfg,
		session:     session.New(awsConfig),
	}
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

	return cfnstack.NewProvisioner(c.StackName(), c.WorkerDeploymentSettings().StackTags(), stackPolicyBody, c.session)
}

func (c *Cluster) Create() error {
	cfSvc := cloudformation.New(c.session)
	s3Svc := s3.New(c.session)

	uploads := map[string]string{
		"stack.json":      string(c.StackBody),
		"userdata-worker": c.UserDataWorker,
	}

	return c.stackProvisioner().CreateStackAndWait(cfSvc, s3Svc, uploads, c.S3URI)
}

func (c *Cluster) Update() (string, error) {
	cfSvc := cloudformation.New(c.session)
	s3Svc := s3.New(c.session)

	uploads := map[string]string{
		"stack.json":      string(c.StackBody),
		"userdata-worker": c.UserDataWorker,
	}

	updateOutput, err := c.stackProvisioner().UpdateStackAndWait(cfSvc, s3Svc, uploads, c.S3URI)

	return updateOutput, err
}

func (c *Cluster) ValidateStack() (string, error) {
	return c.stackProvisioner().Validate(string(c.StackBody), c.S3URI)
}

func (c *Cluster) Info() (*Info, error) {
	var info Info
	{
		info.Name = c.NodePoolName
	}
	return &info, nil
}

func (c *Cluster) Destroy() error {
	return c.stackProvisioner().Destroy()
}
