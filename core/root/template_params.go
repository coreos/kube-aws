package root

import (
	"fmt"
	controlplane "github.com/kubernetes-incubator/kube-aws/core/controlplane/cluster"
	nodepool "github.com/kubernetes-incubator/kube-aws/core/nodepool/cluster"
)

type TemplateParams struct {
	cluster clusterImpl
}

func (p TemplateParams) ClusterName() string {
	return p.cluster.controlPlane.ClusterName
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

func (p controlPlane) TemplateURL() (string, error) {
	u, err := p.controlPlane.TemplateURL()

	if u == "" || err != nil {
		return "", fmt.Errorf("failed to get TemplateURL for %+v: %v", p.controlPlane.String(), err)
	}

	return u, nil
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

func (c TemplateParams) ControlPlane() NestedStack {
	return controlPlane{
		controlPlane: c.cluster.controlPlane,
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
