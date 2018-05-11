package root

import (
	"bytes"
	"fmt"
	controlplane "github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	etcd "github.com/kubernetes-incubator/kube-aws/core/etcd/config"
	network "github.com/kubernetes-incubator/kube-aws/core/network/config"
	nodepool "github.com/kubernetes-incubator/kube-aws/core/nodepool/config"
	"github.com/kubernetes-incubator/kube-aws/core/root/config"
	"github.com/kubernetes-incubator/kube-aws/core/root/defaults"
	"github.com/kubernetes-incubator/kube-aws/filegen"
	"github.com/kubernetes-incubator/kube-aws/plugin/pluginmodel"
	"path/filepath"
	"text/template"
)

func RenderStack(configPath string) error {

	cluster, err := controlplane.ClusterFromFile(configPath)
	if err != nil {
		return err
	}
	clusterConfig, err := cluster.Config([]*pluginmodel.Plugin{})
	if err != nil {
		return err
	}
	kubeconfig, err := parseKubeconfig(clusterConfig)
	if err != nil {
		return err
	}

	if err := filegen.Render(
		filegen.File(filepath.Join(defaults.AssetsDir, ".gitignore"), []byte("*"), 0644),
		filegen.File(defaults.ControllerTmplFile, controlplane.CloudConfigController, 0644),
		filegen.File(defaults.WorkerTmplFile, controlplane.CloudConfigWorker, 0644),
		filegen.File(defaults.EtcdTmplFile, controlplane.CloudConfigEtcd, 0644),
		filegen.File(defaults.ControlPlaneStackTemplateTmplFile, controlplane.StackTemplateTemplate, 0644),
		filegen.File(defaults.NetworkStackTemplateTmplFile, network.StackTemplateTemplate, 0644),
		filegen.File(defaults.EtcdStackTemplateTmplFile, etcd.StackTemplateTemplate, 0644),
		filegen.File(defaults.NodePoolStackTemplateTmplFile, nodepool.StackTemplateTemplate, 0644),
		filegen.File(defaults.RootStackTemplateTmplFile, config.StackTemplateTemplate, 0644),
		filegen.File("kubeconfig", kubeconfig, 0600),
	); err != nil {
		return err
	}

	return nil
}

func parseKubeconfig(clusterConfig *controlplane.Config) ([]byte, error) {

	tmpl, err := template.New("kubeconfig.yaml").Parse(string(controlplane.KubeConfigTemplate))
	if err != nil {
		return nil, fmt.Errorf("failed to parse default kubeconfig template: %v", err)
	}

	var kubeconfig bytes.Buffer
	if err := tmpl.Execute(&kubeconfig, clusterConfig); err != nil {
		return nil, fmt.Errorf("failed to render kubeconfig: %v", err)
	}
	return kubeconfig.Bytes(), nil
}
