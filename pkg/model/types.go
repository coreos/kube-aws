package model

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/kubernetes-incubator/kube-aws/cfnstack"
	"github.com/kubernetes-incubator/kube-aws/credential"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"github.com/kubernetes-incubator/kube-aws/provisioner"
)

type tmplCtx = interface{}

type Stack struct {
	archivedFiles   []provisioner.RemoteFileSpec
	NodeProvisioner *provisioner.Provisioner

	StackName   string
	S3URI       string
	ClusterName string
	Region      api.Region

	Config         *Config
	NodePoolConfig *NodePoolConfig

	tmplCtx

	api.StackTemplateOptions
	UserData          map[string]api.UserData
	CfnInitConfigSets map[string]interface{}
	ExtraCfnResources map[string]interface{}
	ExtraCfnTags      map[string]interface{}
	ExtraCfnOutputs   map[string]interface{}

	AssetsConfig *credential.CompactAssets
	assets       cfnstack.Assets
}

type ec2Service interface {
	CreateVolume(*ec2.CreateVolumeInput) (*ec2.Volume, error)
	DescribeVpcs(*ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error)
	DescribeSubnets(*ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error)
	DescribeKeyPairs(*ec2.DescribeKeyPairsInput) (*ec2.DescribeKeyPairsOutput, error)
}

type StackTemplateGetter interface {
	GetTemplate(input *cloudformation.GetTemplateInput) (*cloudformation.GetTemplateOutput, error)
}

type Context struct {
	Session *session.Session

	ProvidedEncryptService  credential.KMSEncryptionService
	ProvidedCFInterrogator  cfnstack.CFInterrogator
	ProvidedEC2Interrogator cfnstack.EC2Interrogator
	StackTemplateGetter     StackTemplateGetter
}

// An EtcdTmplCtx contains configuration settings/options mixed with existing state in a way that can be
// consumed by stack and cloud-config templates.
type EtcdTmplCtx struct {
	*Stack
	*Config
	api.EtcdExistingState
	EtcdNodes []EtcdNode
}

// ControllerTmplCtx is used for rendering controller stack and userdata
type ControllerTmplCtx struct {
	*Stack
	*Config
	VPC     api.VPC
	Subnets api.Subnets
}

// WorkerTmplCtx is used for rendering worker stacks and userdata
type WorkerTmplCtx struct {
	*Stack
	*NodePoolConfig
}

type NetworkTmplCtx struct {
	*Stack
	*Config
	WorkerNodePools []WorkerTmplCtx
}
