package render

import (
	"bytes"
	"fmt"
	controlplane "github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	nodepool "github.com/kubernetes-incubator/kube-aws/core/nodepool/config"
	"github.com/kubernetes-incubator/kube-aws/core/root/config"
	"github.com/kubernetes-incubator/kube-aws/core/root/defaults"
	"github.com/kubernetes-incubator/kube-aws/filegen"
	"path/filepath"
	"text/template"
)

type StackRenderer interface {
	RenderFiles() error
}

type stackRendererImpl struct {
	cfg *controlplane.Config
}

func NewStackRenderer(c *controlplane.Config) StackRenderer {
	return stackRendererImpl{
		cfg: c,
	}
}

func (r stackRendererImpl) RenderFiles() error {
	tmpl, err := template.New("kubeconfig.yaml").Parse(string(controlplane.KubeConfigTemplate))
	if err != nil {
		return fmt.Errorf("Failed to parse default kubeconfig template: %v", err)
	}
	var kubeconfig bytes.Buffer
	if err := tmpl.Execute(&kubeconfig, r.cfg); err != nil {
		return fmt.Errorf("Failed to render kubeconfig: %v", err)
	}

	if err := filegen.Render(
		filegen.File(filepath.Join(defaults.AssetsDir, ".gitignore"), []byte("*"), 0644),
		filegen.File(defaults.ControllerTmplFile, controlplane.CloudConfigController, 0644),
		filegen.File(defaults.WorkerTmplFile, controlplane.CloudConfigWorker, 0644),
		filegen.File(defaults.EtcdTmplFile, controlplane.CloudConfigEtcd, 0644),
		filegen.File(defaults.ControlPlaneStackTemplateTmplFile, controlplane.StackTemplateTemplate, 0644),
		filegen.File(defaults.NodePoolStackTemplateTmplFile, nodepool.StackTemplateTemplate, 0644),
		filegen.File(defaults.RootStackTemplateTmplFile, config.StackTemplateTemplate, 0644),
		filegen.File("kubeconfig", kubeconfig.Bytes(), 0600),
	); err != nil {
		return err
	}

	return nil
}
