package config

//go:generate go run ../../../codegen/templates_gen.go StackTemplateTemplate=stack-template.json
//go:generate gofmt -w templates.go

import (
	"fmt"
	controlplane "github.com/coreos/kube-aws/core/controlplane/config"
	nodepool "github.com/coreos/kube-aws/core/nodepool/config"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type UnmarshalledConfig struct {
	controlplane.Cluster `yaml:",inline"`
	WorkerConfig         `yaml:"worker,omitempty"`
}

type WorkerConfig struct {
	NodePools []*nodepool.ProvidedConfig `yaml:"nodePools,omitempty"`
}

type Config struct {
	*controlplane.Cluster
	NodePools []*nodepool.ProvidedConfig
}

func newDefaultUnmarshalledConfig() *UnmarshalledConfig {
	return &UnmarshalledConfig{
		Cluster: *controlplane.NewDefaultCluster(),
		WorkerConfig: WorkerConfig{
			NodePools: []*nodepool.ProvidedConfig{},
		},
	}
}

func ConfigFromBytes(data []byte) (*Config, error) {
	c := newDefaultUnmarshalledConfig()
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}
	c.HyperkubeImage.Tag = c.K8sVer

	cpCluser := &c.Cluster
	if err := cpCluser.Load(); err != nil {
		return nil, err
	}

	cpConfig, err := cpCluser.Config()
	if err != nil {
		return nil, err
	}

	nodePools := c.NodePools
	for i, np := range nodePools {
		if err := np.Load(cpConfig); err != nil {
			return nil, fmt.Errorf("invalid node pool at index %d: %v", i, err)
		}
	}

	return &Config{Cluster: cpCluser, NodePools: nodePools}, nil
}

func ConfigFromBytesWithEncryptService(data []byte, encryptService controlplane.EncryptService) (*Config, error) {
	c, err := ConfigFromBytes(data)
	if err != nil {
		return nil, err
	}
	c.ProvidedEncryptService = encryptService
	return c, nil
}

func ConfigFromFile(configPath string) (*Config, error) {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	c, err := ConfigFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("file %s: %v", configPath, err)
	}

	return c, nil
}

func (c *Config) RootStackName() string {
	return c.ClusterName
}
