package root

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/gobuffalo/packr"
	"github.com/kubernetes-incubator/kube-aws/builtin"
	controlplane "github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	"github.com/kubernetes-incubator/kube-aws/filegen"
	"github.com/kubernetes-incubator/kube-aws/plugin/pluginmodel"
	"os"
	"strings"
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
	kubeconfig, err := generateKubeconfig(clusterConfig)
	if err != nil {
		return err
	}

	ignoredWords := []string{
		"etcdadm",
		"kubeconfig.tmpl",
		"cluster.yaml.tmpl",
	}

	if err := builtin.Box().Walk(func(path string, file packr.File) error {
		for _, f := range ignoredWords {
			if strings.Contains(path, f) {
				fmt.Fprintf(os.Stderr, "ignored %s\n", path)
				return nil
			}
		}
		content, err := builtin.Box().MustBytes(path)
		if err != nil {
			return err
		}
		gen := filegen.File(path, content, 0644)
		return filegen.Render(gen)
	}); err != nil {
		return err
	}

	if err := filegen.Render(
		filegen.File("kubeconfig", kubeconfig, 0600),
	); err != nil {
		return err
	}

	return nil
}

func generateKubeconfig(clusterConfig *controlplane.Config) ([]byte, error) {

	tmpl, err := template.New("kubeconfig.yaml").Parse(builtin.String("kubeconfig.tmpl"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse default kubeconfig template: %v", err)
	}

	var kubeconfig bytes.Buffer
	if err := tmpl.Execute(&kubeconfig, clusterConfig); err != nil {
		return nil, fmt.Errorf("failed to render kubeconfig: %v", err)
	}
	return kubeconfig.Bytes(), nil
}
