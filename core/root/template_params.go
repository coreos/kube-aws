package root

import (
	"fmt"

	controlplane "github.com/kubernetes-incubator/kube-aws/core/controlplane/cluster"
	config "github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	etcd "github.com/kubernetes-incubator/kube-aws/core/etcd/cluster"
	network "github.com/kubernetes-incubator/kube-aws/core/network/cluster"
	nodepool "github.com/kubernetes-incubator/kube-aws/core/nodepool/cluster"
)

// TemplateParams is the set of parameters exposed for templating cfn stack template for the root stack
type TemplateParams struct {
	cluster clusterImpl
}

func (p TemplateParams) ExtraCfnResources() map[string]interface{} {
	return p.cluster.ExtraCfnResources
}

func (p TemplateParams) ClusterName() string {
	return p.cluster.controlPlane.ClusterName
}

func (p TemplateParams) KubeAwsVersion() string {
	return controlplane.VERSION
}

func (p TemplateParams) CloudWatchLogging() config.CloudWatchLogging {
	return p.cluster.controlPlane.CloudWatchLogging
}

func (p TemplateParams) KubeDnsMasq() config.KubeDns {
	return p.cluster.controlPlane.KubeDns
}

func newTemplateParams(c clusterImpl) TemplateParams {
	return TemplateParams{
		cluster: c,
	}
}

type NestedStack interface {
	Name() string
	Tags() map[string]string
	TemplateURL() (string, error)
	NeedToExportIAMroles() bool
}

type networkStack struct {
	network *network.Cluster
}

func (p networkStack) Name() string {
	return p.network.NestedStackName()
}

func (p networkStack) Tags() map[string]string {
	return p.network.StackTags
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
	etcd *etcd.Cluster
}

func (p etcdStack) Name() string {
	return p.etcd.NestedStackName()
}

func (p etcdStack) Tags() map[string]string {
	return p.etcd.StackTags
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
	controlPlane *controlplane.Cluster
}

func (p controlPlane) Name() string {
	return p.controlPlane.NestedStackName()
}

func (p controlPlane) Tags() map[string]string {
	return p.controlPlane.StackTags
}

func (p controlPlane) NeedToExportIAMroles() bool {
	return p.controlPlane.Controller.IAMConfig.InstanceProfile.Arn == ""
}

func (p controlPlane) TemplateURL() (string, error) {
	u, err := p.controlPlane.TemplateURL()

	if u == "" || err != nil {
		return "", fmt.Errorf("failed to get TemplateURL for %+v: %v", p.controlPlane.String(), err)
	}

	return u, nil
}

func (p controlPlane) CloudWatchLogging() config.CloudWatchLogging {
	return p.controlPlane.CloudWatchLogging
}

func (p controlPlane) KubeDns() config.KubeDns {
	return p.controlPlane.KubeDns
}

type nodePool struct {
	nodePool *nodepool.Cluster
}

func (p nodePool) Name() string {
	return p.nodePool.NestedStackName()
}

func (p nodePool) Tags() map[string]string {
	return p.nodePool.StackTags
}

func (p nodePool) TemplateURL() (string, error) {
	u, err := p.nodePool.TemplateURL()

	if err != nil || u == "" {
		return "", fmt.Errorf("failed to get template url: %v", err)
	}

	return u, nil
}

func (p nodePool) CloudWatchLogging() config.CloudWatchLogging {
	return p.nodePool.CloudWatchLogging
}

func (p nodePool) KubeDns() config.KubeDns {
	return p.nodePool.KubeDns
}

func (p nodePool) NeedToExportIAMroles() bool {
	return p.nodePool.IAMConfig.InstanceProfile.Arn == ""
}

func (c TemplateParams) ControlPlane() controlPlane {
	return controlPlane{
		controlPlane: c.cluster.controlPlane,
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

func (c TemplateParams) NodePools() []NestedStack {
	nps := []NestedStack{}
	for _, np := range c.cluster.nodePools {
		nps = append(nps, nodePool{
			nodePool: np,
		})
	}
	return nps
}
