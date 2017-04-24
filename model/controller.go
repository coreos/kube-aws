package model

import (
	"errors"
	"fmt"
)

// TODO Merge this with NodePoolConfig
type Controller struct {
	AutoScalingGroup   AutoScalingGroup  `yaml:"autoScalingGroup,omitempty"`
	ClusterAutoscaler  ClusterAutoscaler `yaml:"clusterAutoscaler,omitempty"`
	EC2Instance        `yaml:",inline"`
	LoadBalancer       ControllerElb       `yaml:"loadBalancer,omitempty"`
	ManagedIamRoleName string              `yaml:"managedIamRoleName,omitempty"`
	SecurityGroupIds   []string            `yaml:"securityGroupIds"`
	Subnets            []Subnet            `yaml:"subnets,omitempty"`
	CustomFiles        []CustomFile        `yaml:"customFiles,omitempty"`
	CustomSystemdUnits []CustomSystemdUnit `yaml:"customSystemdUnits,omitempty"`
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
	if err := c.AutoScalingGroup.Valid(); err != nil {
		return err
	}

	if c.ClusterAutoscaler.Enabled() {
		return errors.New("cluster-autoscaler can't be enabled for a control plane because " +
			"allowing so for a group of controller nodes spreading over 2 or more availability zones " +
			"results in unreliability while scaling nodes out.")
	}
	return nil
}

type ControllerElb struct {
	Private bool
	Subnets []Subnet
}
