package model

import (
	"fmt"
)

type NodePoolConfig struct {
	AutoScalingGroup   AutoScalingGroup  `yaml:"autoScalingGroup,omitempty"`
	ClusterAutoscaler  ClusterAutoscaler `yaml:"clusterAutoscaler"`
	SpotFleet          SpotFleet         `yaml:"spotFleet,omitempty"`
	Count              *int              `yaml:"count,omitempty"`
	CreateTimeout      string            `yaml:"createTimeout,omitempty"`
	InstanceType       string            `yaml:"instanceType,omitempty"`
	ManagedIamRoleName string            `yaml:"managedIamRoleName,omitempty"`
	RootVolume         `yaml:",inline"`
	SpotPrice          string                 `yaml:"spotPrice,omitempty"`
	SecurityGroupIds   []string               `yaml:"securityGroupIds,omitempty"`
	Tenancy            string                 `yaml:"tenancy,omitempty"`
	CustomSettings     map[string]interface{} `yaml:"customSettings,omitempty"`
	VolumeMounts       []VolumeMount          `yaml:"volumeMounts,omitempty"`
}

type ClusterAutoscaler struct {
	MinSize int `yaml:"minSize"`
	MaxSize int `yaml:"maxSize"`
}

func (a ClusterAutoscaler) Enabled() bool {
	return a.MinSize > 0
}

type LaunchSpecification struct {
	WeightedCapacity int    `yaml:"weightedCapacity,omitempty"`
	InstanceType     string `yaml:"instanceType,omitempty"`
	SpotPrice        string `yaml:"spotPrice,omitempty"`
	RootVolume       `yaml:",inline"`
}

func NewDefaultNodePoolConfig() NodePoolConfig {
	c := 1
	return NodePoolConfig{
		SpotFleet:     newDefaultSpotFleet(),
		Count:         &c,
		CreateTimeout: "PT15M",
		InstanceType:  "t2.medium",
		RootVolume: RootVolume{
			RootVolumeType: "gp2",
			RootVolumeIOPS: 0,
			RootVolumeSize: 30,
		},
		SecurityGroupIds: []string{},
		Tenancy:          "default",
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

func NewLaunchSpecification(weightedCapacity int, instanceType string) LaunchSpecification {
	return LaunchSpecification{
		WeightedCapacity: weightedCapacity,
		InstanceType:     instanceType,
		RootVolume: RootVolume{
			RootVolumeSize: 0,
			RootVolumeIOPS: 0,
			RootVolumeType: "",
		},
	}
}

func (c NodePoolConfig) LogicalName() string {
	return "Workers"
}

func (c NodePoolConfig) Valid() error {
	// one is the default WorkerCount
	if c.Count != nil && *c.Count != 1 && (c.AutoScalingGroup.MinSize != nil && *c.AutoScalingGroup.MinSize != 0 || c.AutoScalingGroup.MaxSize != 0) {
		return fmt.Errorf("`worker.autoScalingGroup.minSize` and `worker.autoScalingGroup.maxSize` can only be specified without `count`=%d", *c.Count)
	}

	if err := c.AutoScalingGroup.Valid(); err != nil {
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

	if err := c.SpotFleet.Valid(); c.SpotFleet.Enabled() && err != nil {
		return err
	}

	if err := ValidateVolumeMounts(c.VolumeMounts); err != nil {
		return err
	}

	if c.InstanceType == "t2.micro" || c.InstanceType == "t2.nano" {
		fmt.Println(`WARNING: instance types "t2.nano" and "t2.micro" are not recommended. See https://github.com/coreos/kube-aws/issues/258 for more information`)
	}

	return nil
}

func (c NodePoolConfig) MinCount() int {
	if c.AutoScalingGroup.MinSize == nil {
		return *c.Count
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

func (c LaunchSpecification) Valid() error {
	if err := c.RootVolume.Validate(); err != nil {
		return err
	}

	return nil
}
