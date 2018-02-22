package render

import (
	"github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	"github.com/kubernetes-incubator/kube-aws/core/root/defaults"
	"os"
)

type CredentialsRenderer interface {
	RenderCredentials(config.CredentialsOptions) error
}

type credentialsRendererImpl struct {
	c *config.Cluster
}

func NewCredentialsRenderer(c *config.Cluster) CredentialsRenderer {
	return credentialsRendererImpl{
		c: c,
	}
}

func (r credentialsRendererImpl) RenderCredentials(renderCredentialsOpts config.CredentialsOptions) error {
	cluster := r.c
	dir := defaults.AssetsDir
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	_, err := cluster.NewAssetsOnDisk(dir, renderCredentialsOpts)
	if err != nil {
		return err
	}

	return nil
}
