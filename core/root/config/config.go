package config

import (
	"errors"
	"fmt"
	"io/ioutil"

	controlplane "github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	nodepool "github.com/kubernetes-incubator/kube-aws/core/nodepool/config"
	"github.com/kubernetes-incubator/kube-aws/plugin"
	"github.com/kubernetes-incubator/kube-aws/plugin/pluginmodel"
	"gopkg.in/yaml.v2"
)

type UnmarshalledConfig struct {
	controlplane.Cluster `yaml:",inline"`
	Worker               `yaml:"worker,omitempty"`
}

type Worker struct {
	APIEndpointName string                     `yaml:"apiEndpointName,omitempty"`
	NodePools       []*nodepool.ProvidedConfig `yaml:"nodePools,omitempty"`
}

type Config struct {
	*controlplane.Cluster
	NodePools []*nodepool.ProvidedConfig
	Plugins   []*pluginmodel.Plugin
}

func newDefaultUnmarshalledConfig() *UnmarshalledConfig {
	return &UnmarshalledConfig{
		Cluster: *controlplane.NewDefaultCluster(),
		Worker: Worker{
			NodePools: []*nodepool.ProvidedConfig{},
		},
	}
}

func ConfigFromBytes(data []byte, plugins []*pluginmodel.Plugin) (*Config, error) {
	c := newDefaultUnmarshalledConfig()
	if err := yaml.UnmarshalStrict(data, c); err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}
	c.HyperkubeImage.Tag = c.K8sVer

	cpCluster := &c.Cluster
	if err := cpCluster.Load(); err != nil {
		return nil, err
	}

	cpConfig, err := cpCluster.Config(plugins)
	if err != nil {
		return nil, err
	}

	nodePools := c.NodePools

	anyNodePoolIsMissingAPIEndpointName := true
	for _, np := range nodePools {
		if np.APIEndpointName == "" {
			anyNodePoolIsMissingAPIEndpointName = true
			break
		}
	}

	if len(cpConfig.APIEndpoints) > 1 && c.Worker.APIEndpointName == "" && anyNodePoolIsMissingAPIEndpointName {
		return nil, errors.New("worker.apiEndpointName must not be empty when there're 2 or more API endpoints under the key `apiEndpoints` and one of worker.nodePools[] are missing apiEndpointName")
	}

	if c.Worker.APIEndpointName != "" {
		if _, err := cpConfig.APIEndpoints.FindByName(c.APIEndpointName); err != nil {
			return nil, fmt.Errorf("invalid value for worker.apiEndpointName: no API endpoint named \"%s\" found", c.APIEndpointName)
		}
	}

	for i, np := range nodePools {
		if np == nil {
			return nil, fmt.Errorf("Empty nodepool definition found at index %d", i)
		}
		if err := np.Taints.Validate(); err != nil {
			return nil, fmt.Errorf("invalid taints for node pool at index %d: %v", i, err)
		}

		if np.APIEndpointName == "" {
			if c.Worker.APIEndpointName == "" {
				if len(cpConfig.APIEndpoints) > 1 {
					return nil, errors.New("worker.apiEndpointName can be omitted only when there's only 1 api endpoint under apiEndpoints")
				}
				np.APIEndpointName = cpConfig.APIEndpoints.GetDefault().Name
			} else {
				np.APIEndpointName = c.Worker.APIEndpointName
			}
		}

		if err := np.Load(cpConfig); err != nil {
			return nil, fmt.Errorf("invalid node pool at index %d: %v", i, err)
		}

		if np.Autoscaling.ClusterAutoscaler.Enabled && !cpConfig.Addons.ClusterAutoscaler.Enabled {
			return nil, errors.New("Autoscaling with cluster-autoscaler can't be enabled for node pools because " +
				"you didn't enabled the cluster-autoscaler addon. Enable it by turning on `addons.clusterAutoscaler.enabled`")
		}
	}

	cfg := &Config{Cluster: cpCluster, NodePools: nodePools}

	cfg.Plugins = plugins

	return cfg, nil
}

func ConfigFromBytesWithEncryptService(data []byte, plugins []*pluginmodel.Plugin, encryptService controlplane.EncryptService) (*Config, error) {
	c, err := ConfigFromBytes(data, plugins)
	if err != nil {
		return nil, err
	}
	c.ProvidedEncryptService = encryptService

	// Uses the same encrypt service for node pools for consistency
	for _, p := range c.NodePools {
		p.ProvidedEncryptService = encryptService
	}

	return c, nil
}

func ConfigFromFile(configPath string) (*Config, error) {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	plugins, err := plugin.LoadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to load plugins: %v", err)
	}

	c, err := ConfigFromBytes(data, plugins)
	if err != nil {
		return nil, fmt.Errorf("file %s: %v", configPath, err)
	}

	return c, nil
}

func (c *Config) RootStackName() string {
	return c.ClusterName
}
