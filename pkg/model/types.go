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

// UserDataEtcd is here for backward-compatibility.
// You should use `Userdata.Etcd` instead in your templates.
func (c EtcdTmplCtx) UserDataEtcd() *api.UserData {
	return c.GetUserData("Etcd")
}

// ControllerTmplCtx is used for rendering controller stack and userdata
type ControllerTmplCtx struct {
	*Stack
	*Config
	VPC     api.VPC
	Subnets api.Subnets
}

// UserDataController is here for backward-compatibility.
// You should use `Userdata.Controller` instead in your templates.
func (c ControllerTmplCtx) UserDataController() *api.UserData {
	return c.GetUserData("Controller")
}

func (c ControllerTmplCtx) MinControllerCount() int {
	return c.Controller.MinControllerCount()
}

func (c ControllerTmplCtx) MaxControllerCount() int {
	return c.Controller.MaxControllerCount()
}

func (c ControllerTmplCtx) ControllerRollingUpdateMinInstancesInService() int {
	return c.Controller.ControllerRollingUpdateMinInstancesInService()
}

// WorkerTmplCtx is used for rendering worker stacks and userdata
type WorkerTmplCtx struct {
	*Stack
	*NodePoolConfig
}

// UserDataWorker is here for backward-compatibility.
// You should use `Userdata.Worker` instead in your templates.
func (c WorkerTmplCtx) UserDataWorker() *api.UserData {
	return c.GetUserData("Worker")
}

type NetworkTmplCtx struct {
	*Stack
	*Config
	WorkerNodePools []WorkerTmplCtx
}
