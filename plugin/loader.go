package plugin

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/go-yaml/yaml"
	"github.com/kubernetes-incubator/kube-aws/pkg/clusterapi"
	"os"
)

type Loader struct {
}

func NewLoader() *Loader {
	return &Loader{}
}

func (l Loader) Load() ([]*clusterapi.Plugin, error) {
	plugins := []*clusterapi.Plugin{}
	fileInfos, _ := ioutil.ReadDir("plugins/")
	for _, f := range fileInfos {
		if f.IsDir() {
			p, err := l.TryToLoadPluginFromDir(filepath.Join("plugins", f.Name()))
			if err != nil {
				return []*clusterapi.Plugin{}, fmt.Errorf("Failed to load plugin from the directory %s: %v", f.Name(), err)
			}
			plugins = append(plugins, p)
			fmt.Fprintf(os.Stderr, "loaded plugin %v\n", p)
		}
	}
	return plugins, nil
}

func (l Loader) TryToLoadPluginFromDir(path string) (*clusterapi.Plugin, error) {
	p, err := PluginFromFile(filepath.Join(path, "plugin.yaml"))
	if err != nil {
		return nil, fmt.Errorf("Failed to load plugin from %s: %v", path, err)
	}
	return p, nil
}

func PluginFromFile(path string) (*clusterapi.Plugin, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to read file %s: %v", path, err)
	}

	c, err := PluginFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("Failed while processing file %s: %v", path, err)
	}

	return c, nil
}

func PluginFromBytes(data []byte) (*clusterapi.Plugin, error) {
	fmt.Fprintf(os.Stderr, "plugin bytes %s\n", string(data))
	p := &clusterapi.Plugin{}
	if err := yaml.UnmarshalStrict(data, p); err != nil {
		return nil, fmt.Errorf("Failed to parse as yaml: %v", err)
	}
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("Failed to validate plugin \"%s\": %v", p.Name, err)
	}
	return p, nil
}

func LoadAll() ([]*clusterapi.Plugin, error) {
	loaders := []*Loader{
		NewLoader(),
	}

	plugins := []*clusterapi.Plugin{}
	for _, l := range loaders {
		ps, err := l.Load()
		if err != nil {
			return plugins, fmt.Errorf("Failed to load plugins: %v", err)
		}
		plugins = append(plugins, ps...)
	}
	return plugins, nil
}
