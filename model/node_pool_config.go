package model

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/logger"
)

type NodePoolConfig struct {
	Autoscaling               Autoscaling      `yaml:"autoscaling,omitempty"`
	AutoScalingGroup          AutoScalingGroup `yaml:"autoScalingGroup,omitempty"`
	SpotFleet                 SpotFleet        `yaml:"spotFleet,omitempty"`
	EC2Instance               `yaml:",inline"`
	IAMConfig                 IAMConfig              `yaml:"iam,omitempty"`
	SpotPrice                 string                 `yaml:"spotPrice,omitempty"`
	SecurityGroupIds          []string               `yaml:"securityGroupIds,omitempty"`
	CustomSettings            map[string]interface{} `yaml:"customSettings,omitempty"`
	VolumeMounts              []VolumeMount          `yaml:"volumeMounts,omitempty"`
	Raid0Mounts               []Raid0Mount           `yaml:"raid0Mounts,omitempty"`
	UnknownKeys               `yaml:",inline"`
	NodeSettings              `yaml:",inline"`
	NodeStatusUpdateFrequency string              `yaml:"nodeStatusUpdateFrequency"`
	CustomFiles               []CustomFile        `yaml:"customFiles,omitempty"`
	CustomSystemdUnits        []CustomSystemdUnit `yaml:"customSystemdUnits,omitempty"`
	Gpu                       Gpu                 `yaml:"gpu"`
}

type ClusterAutoscaler struct {
	Enabled     bool `yaml:"enabled,omitempty"`
	UnknownKeys `yaml:",inline"`
}

func (a ClusterAutoscaler) AutoDiscoveryTagKey() string {
	return "k8s.io/cluster-autoscaler/enabled"
}

func NewDefaultNodePoolConfig() NodePoolConfig {
	return NodePoolConfig{
		SpotFleet: newDefaultSpotFleet(),
		EC2Instance: EC2Instance{
			Count:         1,
			CreateTimeout: "PT15M",
			InstanceType:  "t2.medium",
			RootVolume: RootVolume{
				Type: "gp2",
				IOPS: 0,
				Size: 30,
			},
			Tenancy: "default",
		},
		NodeSettings:     newNodeSettings(),
		SecurityGroupIds: []string{},
		Gpu:              newDefaultGpu(),
	}
}

func newDefaultSpotFleet() SpotFleet {
	return SpotFleet{
		SpotPrice:          "0.06",
		UnitRootVolumeSize: 30,
		RootVolumeType:     "gp2",
		LaunchSpecifications: []LaunchSpecification{
			NewLaunchSpecification(1, "c4.large"),
			NewLaunchSpecification(2, "c4.xlarge"),
		},
	}
}

func (c NodePoolConfig) LogicalName() string {
	return "Workers"
}

func (c NodePoolConfig) Validate(experimentalGpuSupportEnabled bool) error {
	// one is the default WorkerCount
	if c.Count != 1 && (c.AutoScalingGroup.MinSize != nil && *c.AutoScalingGroup.MinSize != 0 || c.AutoScalingGroup.MaxSize != 0) {
		return fmt.Errorf("`worker.autoScalingGroup.minSize` and `worker.autoScalingGroup.maxSize` can only be specified without `count`=%d", c.Count)
	}

	if err := c.AutoScalingGroup.Validate(); err != nil {
		return err
	}

	if c.Tenancy != "default" && c.SpotFleet.Enabled() {
		return fmt.Errorf("selected worker tenancy (%s) is incompatible with spot fleet", c.Tenancy)
	}

	if c.Tenancy != "default" && c.SpotPrice != "" {
		return fmt.Errorf("selected worker tenancy (%s) is incompatible with spot instances", c.Tenancy)
	}

	if err := c.RootVolume.Validate(); err != nil {
		return err
	}

	if err := c.SpotFleet.Validate(); c.SpotFleet.Enabled() && err != nil {
		return err
	}

	if err := ValidateVolumeMounts(c.VolumeMounts); err != nil {
		return err
	}

	// c.VolumeMounts are supplied to check for device and path overlaps with contents of c.Raid0Mounts.
	if err := ValidateRaid0Mounts(c.VolumeMounts, c.Raid0Mounts); err != nil {
		return err
	}

	if c.InstanceType == "t2.micro" || c.InstanceType == "t2.nano" {
		logger.Warnf(`instance types "t2.nano" and "t2.micro" are not recommended. See https://github.com/kubernetes-incubator/kube-aws/issues/258 for more information`)
	}

	if err := c.IAMConfig.Validate(); err != nil {
		return err
	}

	if err := c.Gpu.Validate(c.InstanceType, experimentalGpuSupportEnabled); err != nil {
		return err
	}

	return nil
}

func (c NodePoolConfig) MinCount() int {
	if c.AutoScalingGroup.MinSize == nil {
		return c.Count
	}
	return *c.AutoScalingGroup.MinSize
}

func (c NodePoolConfig) MaxCount() int {
	if c.AutoScalingGroup.MaxSize == 0 {
		return c.MinCount()
	}
	return c.AutoScalingGroup.MaxSize
}

func (c NodePoolConfig) RollingUpdateMinInstancesInService() int {
	if c.AutoScalingGroup.RollingUpdateMinInstancesInService == nil {
		if c.MaxCount() > 0 {
			return c.MaxCount() - 1
		}
		return 0
	}
	return *c.AutoScalingGroup.RollingUpdateMinInstancesInService
}
