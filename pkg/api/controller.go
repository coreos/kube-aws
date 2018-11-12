package api

import (
	"errors"
	"fmt"
)

// TODO Merge this with WorkerNodePool
type Controller struct {
	AutoScalingGroup   AutoScalingGroup `yaml:"autoScalingGroup,omitempty"`
	Autoscaling        Autoscaling      `yaml:"autoscaling,omitempty"`
	EC2Instance        `yaml:",inline"`
	LoadBalancer       ControllerElb       `yaml:"loadBalancer,omitempty"`
	IAMConfig          IAMConfig           `yaml:"iam,omitempty"`
	SecurityGroupIds   []string            `yaml:"securityGroupIds"`
	VolumeMounts       []NodeVolumeMount   `yaml:"volumeMounts,omitempty"`
	Subnets            Subnets             `yaml:"subnets,omitempty"`
	CustomFiles        []CustomFile        `yaml:"customFiles,omitempty"`
	CustomSystemdUnits []CustomSystemdUnit `yaml:"customSystemdUnits,omitempty"`
	NodeSettings       `yaml:",inline"`
	UnknownKeys        `yaml:",inline"`
}

const DefaultControllerCount = 1

func NewDefaultController() Controller {
	return Controller{
		EC2Instance: EC2Instance{
			Count:         DefaultControllerCount,
			CreateTimeout: "PT15M",
			InstanceType:  "t2.medium",
			RootVolume: RootVolume{
				Type: "gp2",
				IOPS: 0,
				Size: 30,
			},
			Tenancy: "default",
		},
		NodeSettings: newNodeSettings(),
	}
}

func (c Controller) LogicalName() string {
	return "Controllers"
}

func (c Controller) LaunchConfigurationLogicalName() string {
	return c.LogicalName() + "LC"
}

func (c Controller) SecurityGroupRefs() []string {
	refs := []string{}

	for _, id := range c.SecurityGroupIds {
		refs = append(refs, fmt.Sprintf(`"%s"`, id))
	}

	refs = append(
		refs,
		`{"Fn::ImportValue" : {"Fn::Sub" : "${NetworkStackName}-ControllerSecurityGroup"}}`,
	)

	return refs
}

func (c Controller) Validate() error {
	rootVolume := c.RootVolume

	if rootVolume.Type == "io1" {
		if rootVolume.IOPS < 100 || rootVolume.IOPS > 20000 {
			return fmt.Errorf("invalid controller.rootVolume.iops: %d", rootVolume.IOPS)
		}
	} else {
		if rootVolume.IOPS != 0 {
			return fmt.Errorf("invalid controller.rootVolume.iops for type \"%s\": %d", rootVolume.Type, rootVolume.IOPS)
		}

		if rootVolume.Type != "standard" && rootVolume.Type != "gp2" {
			return fmt.Errorf("invalid controller.rootVolume.type: %s in %+v", rootVolume.Type, c)
		}
	}

	if c.Count < 0 {
		return fmt.Errorf("`controller.count` must be zero or greater if specified or otherwrise omitted, but was: %d", c.Count)
	}
	// one is the default Controller.Count
	asg := c.AutoScalingGroup
	if c.Count != DefaultControllerCount && (asg.MinSize != nil && *asg.MinSize != 0 || asg.MaxSize != 0) {
		return errors.New("`controller.autoScalingGroup.minSize` and `controller.autoScalingGroup.maxSize` can only be specified without `controller.count`")
	}

	if err := c.AutoScalingGroup.Validate(); err != nil {
		return err
	}

	if c.Autoscaling.ClusterAutoscaler.Enabled {
		return errors.New("cluster-autoscaler can't be enabled for a control plane because " +
			"allowing so for a group of controller nodes spreading over 2 or more availability zones " +
			"results in unreliability while scaling nodes out.")
	}
	if err := c.IAMConfig.Validate(); err != nil {
		return err
	}
	if err := ValidateVolumeMounts(c.VolumeMounts); err != nil {
		return err
	}
	if len(c.Taints) > 0 {
		return errors.New("`controller.taints` must not be specified because tainting controller nodes breaks the cluster")
	}
	return nil
}

func (c Controller) InstanceProfileRoles() string {
	return fmt.Sprintf(`"Roles": [%s]`, c.InstanceProfileRole())
}

func (c Controller) InstanceProfileRole() string {
	if c.IAMConfig.Role.StrictName && c.IAMConfig.Role.Name != "" {
		return fmt.Sprintf(`"%s"`, c.IAMConfig.Role.Name)
	} else {
		return `{"Ref":"IAMRoleController"}`
	}
}

func (c Controller) MinControllerCount() int {
	if c.AutoScalingGroup.MinSize == nil {
		return c.Count
	}
	return *c.AutoScalingGroup.MinSize
}

func (c Controller) MaxControllerCount() int {
	if c.AutoScalingGroup.MaxSize == 0 {
		return c.Count
	}
	return c.AutoScalingGroup.MaxSize
}

func (c Controller) ControllerRollingUpdateMinInstancesInService() int {
	if c.AutoScalingGroup.RollingUpdateMinInstancesInService == nil {
		return c.MaxControllerCount() - 1
	}
	return *c.AutoScalingGroup.RollingUpdateMinInstancesInService
}

type ControllerElb struct {
	Private bool
	Subnets Subnets
}
