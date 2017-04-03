package model

import (
	"fmt"
)

type NodePoolConfig struct {
	AutoScalingGroup          AutoScalingGroup  `yaml:"autoScalingGroup,omitempty"`
	ClusterAutoscaler         ClusterAutoscaler `yaml:"clusterAutoscaler"`
	SpotFleet                 SpotFleet         `yaml:"spotFleet,omitempty"`
	EC2Instance               `yaml:",inline"`
	ManagedIamRoleName        string `yaml:"managedIamRoleName,omitempty"`
	DeprecatedRootVolume      `yaml:",inline"`
	SpotPrice                 string                 `yaml:"spotPrice,omitempty"`
	SecurityGroupIds          []string               `yaml:"securityGroupIds,omitempty"`
	CustomSettings            map[string]interface{} `yaml:"customSettings,omitempty"`
	VolumeMounts              []VolumeMount          `yaml:"volumeMounts,omitempty"`
	UnknownKeys               `yaml:",inline"`
	NodeStatusUpdateFrequency string `yaml:"nodeStatusUpdateFrequency"`
}

type ClusterAutoscaler struct {
	MinSize     int `yaml:"minSize"`
	MaxSize     int `yaml:"maxSize"`
	UnknownKeys `yaml:",inline"`
}

func (a ClusterAutoscaler) Enabled() bool {
	return a.MinSize > 0
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
		SecurityGroupIds: []string{},
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

func (c NodePoolConfig) Valid() error {
	// one is the default WorkerCount
	if c.Count != 1 && (c.AutoScalingGroup.MinSize != nil && *c.AutoScalingGroup.MinSize != 0 || c.AutoScalingGroup.MaxSize != 0) {
		return fmt.Errorf("`worker.autoScalingGroup.minSize` and `worker.autoScalingGroup.maxSize` can only be specified without `count`=%d", c.Count)
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
		fmt.Println(`WARNING: instance types "t2.nano" and "t2.micro" are not recommended. See https://github.com/kubernetes-incubator/kube-aws/issues/258 for more information`)
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
