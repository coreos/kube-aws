package model

import (
	"errors"
	"fmt"
)

// TODO Merge this with NodePoolConfig
type Controller struct {
	AutoScalingGroup   AutoScalingGroup `yaml:"autoScalingGroup,omitempty"`
	Autoscaling        Autoscaling      `yaml:"autoscaling,omitempty"`
	EC2Instance        `yaml:",inline"`
	LoadBalancer       ControllerElb       `yaml:"loadBalancer,omitempty"`
	IAMConfig          IAMConfig           `yaml:"iam,omitempty"`
	SecurityGroupIds   []string            `yaml:"securityGroupIds"`
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

func (c Controller) SecurityGroupRefs() []string {
	refs := []string{}

	for _, id := range c.SecurityGroupIds {
		refs = append(refs, fmt.Sprintf(`"%s"`, id))
	}

	refs = append(
		refs,
		`{"Ref":"SecurityGroupController"}`,
	)

	return refs
}

func (c Controller) Validate() error {
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
	if len(c.Taints) > 0 {
		return errors.New("`controller.taints` must not be specified because tainting controller nodes breaks the cluster")
	}
	return nil
}

type ControllerElb struct {
	Private bool
	Subnets Subnets
}
