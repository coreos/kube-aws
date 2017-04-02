package config

//go:generate go run ../../../codegen/templates_gen.go StackTemplateTemplate=stack-template.json
//go:generate gofmt -w templates.go

import (
	"fmt"
	"io/ioutil"

	controlplane "github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	nodepool "github.com/kubernetes-incubator/kube-aws/core/nodepool/config"
	"github.com/kubernetes-incubator/kube-aws/model"
	"gopkg.in/yaml.v2"
)

type UnmarshalledConfig struct {
	controlplane.Cluster `yaml:",inline"`
	WorkerConfig         `yaml:"worker,omitempty"`
	model.UnknownKeys    `yaml:",inline"`
}

type WorkerConfig struct {
	NodePools         []*nodepool.ProvidedConfig `yaml:"nodePools,omitempty"`
	model.UnknownKeys `yaml:",inline"`
}

type Config struct {
	*controlplane.Cluster
	NodePools         []*nodepool.ProvidedConfig
	model.UnknownKeys `yaml:",inline"`
}

type unknownKeysSupport interface {
	FailWhenUnknownKeysFound(keyPath string) error
}

type unknownKeyValidation struct {
	unknownKeysSupport
	keyPath string
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

	cpCluster := &c.Cluster
	if err := cpCluster.Load(); err != nil {
		return nil, err
	}

	cpConfig, err := cpCluster.Config()
	if err != nil {
		return nil, err
	}

	nodePools := c.NodePools
	for i, np := range nodePools {
		if err := np.Load(cpConfig); err != nil {
			return nil, fmt.Errorf("invalid node pool at index %d: %v", i, err)
		}

		if err := failFastWhenUnknownKeysFound([]unknownKeyValidation{
			{np, fmt.Sprintf("worker.nodePools[%d]", i)},
			{np.AutoScalingGroup, fmt.Sprintf("worker.nodePools[%d].autoScalingGroup", i)},
			{np.ClusterAutoscaler, fmt.Sprintf("worker.nodePools[%d].clusterAutoscaler", i)},
			{np.SpotFleet, fmt.Sprintf("worker.nodePools[%d].spotFleet", i)},
		}); err != nil {
			return nil, err
		}
	}

	cfg := &Config{Cluster: cpCluster, NodePools: nodePools}

	if err := failFastWhenUnknownKeysFound([]unknownKeyValidation{
		{c, ""},
		{c.WorkerConfig, "worker"},
		{c.Etcd, "etcd"},
		{c.Controller, "controller"},
		{c.Controller.AutoScalingGroup, "controller.autoScalingGroup"},
		{c.Controller.ClusterAutoscaler, "controller.clusterAutoscaler"},
		{c.Experimental, "experimental"},
		{c.Addons, "addons"},
		{c.Addons.Rescheduler, "addons.rescheduler"},
	}); err != nil {
		return nil, err
	}

	return cfg, nil
}

func failFastWhenUnknownKeysFound(vs []unknownKeyValidation) error {
	for _, v := range vs {
		if err := v.unknownKeysSupport.FailWhenUnknownKeysFound(v.keyPath); err != nil {
			return err
		}
	}
	return nil
}

func ConfigFromBytesWithEncryptService(data []byte, encryptService controlplane.EncryptService) (*Config, error) {
	c, err := ConfigFromBytes(data)
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

	c, err := ConfigFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("file %s: %v", configPath, err)
	}

	return c, nil
}

func (c *Config) RootStackName() string {
	return c.ClusterName
}
