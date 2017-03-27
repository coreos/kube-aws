package config

import (
	cfg "github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	"github.com/kubernetes-incubator/kube-aws/model"
)

type WorkerNodePoolConfig struct {
	APIEndpointName      string `yaml:"apiEndpointName,omitempty"`
	model.NodePoolConfig `yaml:",inline"`
}

func NewWorkerNodePoolConfig() WorkerNodePoolConfig {
	return WorkerNodePoolConfig{
		NodePoolConfig: model.NewDefaultNodePoolConfig(),
	}

}

func (c WorkerNodePoolConfig) ValidateInputs() error {
	return nil
}

func (c WorkerNodePoolConfig) Validate() error {
	if err := c.NodePoolConfig.Valid(); err != nil {
		return err
	}
	return nil
}

func (c WorkerNodePoolConfig) WithDefaultsFrom(main cfg.DefaultWorkerSettings) WorkerNodePoolConfig {
	if c.RootVolume.RootVolumeType == "" {
		c.RootVolume.RootVolumeType = main.WorkerRootVolumeType
	}

	if c.RootVolume.RootVolumeIOPS == 0 && c.RootVolume.RootVolumeType == "io1" {
		c.RootVolume.RootVolumeIOPS = main.WorkerRootVolumeIOPS
	}

	if c.SpotFleet.RootVolumeType == "" {
		c.SpotFleet.RootVolumeType = c.RootVolume.RootVolumeType
	}

	if c.RootVolumeSize == 0 {
		c.RootVolumeSize = main.WorkerRootVolumeSize
	}

	if c.Tenancy == "" {
		c.Tenancy = main.WorkerTenancy
	}

	if c.InstanceType == "" {
		c.InstanceType = main.WorkerInstanceType
	}

	if c.CreateTimeout == "" {
		c.CreateTimeout = main.WorkerCreateTimeout
	}

	return c
}
