package root

import (
	"fmt"
	controlplane "github.com/coreos/kube-aws/core/controlplane/cluster"
	nodepool "github.com/coreos/kube-aws/core/nodepool/cluster"
	"strings"
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
	stackName := p.controlPlane.StackName()
	return strings.Title(strings.Replace(stackName, "-", "", -1))
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
	// Convert stack name into something valid as a cfn resource name or
	// we'll end up with cfn errors like "Template format error: Resource name test5-controlplane is non alphanumeric"
	stackName := p.nodePool.StackName()
	return strings.Title(strings.Replace(stackName, "-", "", -1))
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
