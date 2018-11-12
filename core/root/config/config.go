package config

import (
	"fmt"
	"io/ioutil"

	"github.com/go-yaml/yaml"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"github.com/kubernetes-incubator/kube-aws/pkg/model"
	"github.com/kubernetes-incubator/kube-aws/plugin"
	"github.com/kubernetes-incubator/kube-aws/plugin/clusterextension"
	"github.com/pkg/errors"
)

type InitialConfig struct {
	AmiId            string
	AvailabilityZone string
	ClusterName      string
	ExternalDNSName  string
	HostedZoneID     string
	KMSKeyARN        string
	KeyName          string
	NoRecordSet      bool
	Region           api.Region
	S3URI            string
}

type UnmarshalledConfig struct {
	api.Cluster     `yaml:",inline"`
	api.UnknownKeys `yaml:",inline"`
}

type Config struct {
	*model.Config
	NodePools       []*model.NodePoolConfig
	Plugins         []*api.Plugin
	api.UnknownKeys `yaml:",inline"`

	Extras *clusterextension.ClusterExtension
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
		Cluster: *api.NewDefaultCluster(),
	}
}

func unmarshalConfig(data []byte) (*UnmarshalledConfig, error) {
	c := newDefaultUnmarshalledConfig()
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}
	c.HyperkubeImage.Tag = c.K8sVer

	return c, nil
}

func ConfigFromBytes(data []byte, plugins []*api.Plugin) (*Config, error) {
	c, err := unmarshalConfig(data)
	if err != nil {
		return nil, err
	}

	cpCluster := &c.Cluster
	if err := cpCluster.Load(); err != nil {
		return nil, err
	}

	extras := clusterextension.NewExtrasFromPlugins(plugins, c.PluginConfigs)

	opts := api.ClusterOptions{
		S3URI: c.S3URI,
		// TODO
		SkipWait: false,
	}

	cpConfig, err := model.Compile(cpCluster, opts)
	if err != nil {
		return nil, err
	}

	nodePools := c.NodePools

	nps := []*model.NodePoolConfig{}
	for i, np := range nodePools {
		npConf, err := model.NodePoolCompile(np, cpConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid node pool at index %d", i)
		}

		if err := failFastWhenUnknownKeysFound([]unknownKeyValidation{
			{np, fmt.Sprintf("worker.nodePools[%d]", i)},
			{np.AutoScalingGroup, fmt.Sprintf("worker.nodePools[%d].autoScalingGroup", i)},
			{np.Autoscaling.ClusterAutoscaler, fmt.Sprintf("worker.nodePools[%d].autoscaling.clusterAutoscaler", i)},
			{np.SpotFleet, fmt.Sprintf("worker.nodePools[%d].spotFleet", i)},
		}); err != nil {
			return nil, err
		}

		nps = append(nps, npConf)
	}

	cfg := &Config{Config: cpConfig, NodePools: nps}

	validations := []unknownKeyValidation{
		{c, ""},
		{c.Worker, "worker"},
		{c.Etcd, "etcd"},
		{c.Etcd.RootVolume, "etcd.rootVolume"},
		{c.Etcd.DataVolume, "etcd.dataVolume"},
		{c.Controller, "controller"},
		{c.Controller.AutoScalingGroup, "controller.autoScalingGroup"},
		{c.Controller.Autoscaling.ClusterAutoscaler, "controller.autoscaling.clusterAutoscaler"},
		{c.Controller.RootVolume, "controller.rootVolume"},
		{c.Experimental, "experimental"},
		{c.Addons, "addons"},
		{c.Addons.Rescheduler, "addons.rescheduler"},
		{c.Addons.ClusterAutoscaler, "addons.clusterAutoscaler"},
		{c.Addons.MetricsServer, "addons.metricsServer"},
	}

	for i, np := range c.Worker.NodePools {
		validations = append(validations, unknownKeyValidation{np, fmt.Sprintf("worker.nodePools[%d]", i)})
		validations = append(validations, unknownKeyValidation{np.RootVolume, fmt.Sprintf("worker.nodePools[%d].rootVolume", i)})

	}

	for i, endpoint := range c.APIEndpointConfigs {
		validations = append(validations, unknownKeyValidation{endpoint, fmt.Sprintf("apiEndpoints[%d]", i)})
	}

	if err := failFastWhenUnknownKeysFound(validations); err != nil {
		return nil, err
	}

	cfg.Plugins = plugins
	cfg.Extras = &extras

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
		return nil, errors.Wrapf(err, "failed loading %s: %v", configPath, err)
	}

	return c, nil
}

func (c *Config) RootStackName() string {
	return c.ClusterName
}
