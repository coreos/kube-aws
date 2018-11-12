package root

import (
	"fmt"

	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"github.com/kubernetes-incubator/kube-aws/pkg/model"
)

// TemplateParams is the set of parameters exposed for templating cfn stack template for the root stack
type TemplateParams struct {
	cluster Cluster
}

func (p TemplateParams) ExtraCfnResources() map[string]interface{} {
	return p.cluster.ExtraCfnResources
}

func (p TemplateParams) ClusterName() string {
	return p.cluster.controlPlaneStack.ClusterName
}

func (p TemplateParams) KubeAwsVersion() string {
	return model.VERSION
}

func (p TemplateParams) CloudWatchLogging() api.CloudWatchLogging {
	return p.cluster.controlPlaneStack.Config.CloudWatchLogging
}

func (p TemplateParams) KubeDnsMasq() api.KubeDns {
	return p.cluster.controlPlaneStack.Config.KubeDns
}

func newTemplateParams(c *Cluster) TemplateParams {
	return TemplateParams{
		cluster: *c,
	}
}

type NestedStack interface {
	Name() string
	Tags() map[string]string
	TemplateURL() (string, error)
	NeedToExportIAMroles() bool
}

type networkStack struct {
	network *model.Stack
}

func (p networkStack) Name() string {
	return p.network.NestedStackName()
}

func (p networkStack) Tags() map[string]string {
	return p.network.Config.StackTags
}

func (p networkStack) NeedToExportIAMroles() bool {
	return false
}

func (p networkStack) TemplateURL() (string, error) {
	u, err := p.network.TemplateURL()

	if u == "" || err != nil {
		return "", fmt.Errorf("failed to get TemplateURL for %+v: %v", p.network.String(), err)
	}

	return u, nil
}

type etcdStack struct {
	etcd *model.Stack
}

func (p etcdStack) Name() string {
	return p.etcd.NestedStackName()
}

func (p etcdStack) Tags() map[string]string {
	return p.etcd.Config.StackTags
}

func (p etcdStack) NeedToExportIAMroles() bool {
	return false
}

func (p etcdStack) TemplateURL() (string, error) {
	u, err := p.etcd.TemplateURL()

	if u == "" || err != nil {
		return "", fmt.Errorf("failed to get TemplateURL for %+v: %v", p.etcd.String(), err)
	}

	return u, nil
}

type controlPlane struct {
	controlPlane *model.Stack
}

func (p controlPlane) Name() string {
	return p.controlPlane.NestedStackName()
}

func (p controlPlane) Tags() map[string]string {
	return p.controlPlane.Config.StackTags
}

func (p controlPlane) NeedToExportIAMroles() bool {
	return p.controlPlane.Config.Controller.IAMConfig.InstanceProfile.Arn == ""
}

func (p controlPlane) TemplateURL() (string, error) {
	u, err := p.controlPlane.TemplateURL()

	if u == "" || err != nil {
		return "", fmt.Errorf("failed to get TemplateURL for %+v: %v", p.controlPlane.String(), err)
	}

	return u, nil
}

func (p controlPlane) CloudWatchLogging() api.CloudWatchLogging {
	return p.controlPlane.Config.CloudWatchLogging
}

func (p controlPlane) KubeDns() api.KubeDns {
	return p.controlPlane.Config.KubeDns
}

type nodePool struct {
	nodePool *model.Stack
}

func (p nodePool) Name() string {
	return p.nodePool.NestedStackName()
}

func (p nodePool) Tags() map[string]string {
	return p.nodePool.NodePoolConfig.StackTags
}

func (p nodePool) TemplateURL() (string, error) {
	u, err := p.nodePool.TemplateURL()

	if err != nil || u == "" {
		return "", fmt.Errorf("failed to get template url: %v", err)
	}

	return u, nil
}

func (p nodePool) CloudWatchLogging() api.CloudWatchLogging {
	return p.nodePool.NodePoolConfig.CloudWatchLogging
}

func (p nodePool) KubeDns() api.KubeDns {
	return p.nodePool.NodePoolConfig.KubeDns
}

func (p nodePool) NeedToExportIAMroles() bool {
	return p.nodePool.NodePoolConfig.IAMConfig.InstanceProfile.Arn == ""
}

// returns NodePoolRolling strategy string to be used in stack-template
func (p nodePool) NodePoolRollingStrategy() string {
	return p.nodePool.NodePoolConfig.WorkerNodePool.NodePoolRollingStrategy
}

func (c TemplateParams) ControlPlane() controlPlane {
	return controlPlane{
		controlPlane: c.cluster.controlPlaneStack,
	}
}

func (c TemplateParams) Etcd() etcdStack {
	return etcdStack{
		etcd: c.cluster.Etcd(),
	}
}

func (c TemplateParams) Network() networkStack {
	return networkStack{
		network: c.cluster.Network(),
	}
}

func (c TemplateParams) NodePools() []nodePool {
	nps := []nodePool{}
	for _, np := range c.cluster.nodePoolStacks {
		nps = append(nps, nodePool{
			nodePool: np,
		})
	}
	return nps
}
