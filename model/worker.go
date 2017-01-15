package model

import "fmt"

type Worker struct {
	AutoScalingGroup  `yaml:"autoScalingGroup,omitempty"`
	ClusterAutoscaler ClusterAutoscaler `yaml:"clusterAutoscaler"`
	SpotFleet         `yaml:"spotFleet,omitempty"`
}

type ClusterAutoscaler struct {
	MinSize int `yaml:"minSize"`
	MaxSize int `yaml:"maxSize"`
}

func (a ClusterAutoscaler) Enabled() bool {
	return a.MinSize > 0
}

// UnitRootVolumeSize/IOPS are used for spot fleets instead of WorkerRootVolumeSize/IOPS,
// so that we can make them clearer that they are not default size/iops for each worker node but "size/iops per unit"
// as their names suggest
type SpotFleet struct {
	TargetCapacity       int                   `yaml:"targetCapacity,omitempty"`
	SpotPrice            string                `yaml:"spotPrice,omitempty"`
	IAMFleetRoleARN      string                `yaml:"iamFleetRoleArn,omitempty"`
	RootVolumeType       string                `yaml:"rootVolumeType"`
	UnitRootVolumeSize   int                   `yaml:"unitRootVolumeSize"`
	UnitRootVolumeIOPS   int                   `yaml:"unitRootVolumeIOPS"`
	LaunchSpecifications []LaunchSpecification `yaml:"launchSpecifications,omitempty"`
}

type LaunchSpecification struct {
	WeightedCapacity int    `yaml:"weightedCapacity,omitempty"`
	InstanceType     string `yaml:"instanceType,omitempty"`
	SpotPrice        string `yaml:"spotPrice,omitempty"`
	RootVolumeSize   int    `yaml:"rootVolumeSize,omitempty"`
	RootVolumeType   string `yaml:"rootVolumeType,omitempty"`
	RootVolumeIOPS   int    `yaml:"rootVolumeIOPS,omitempty"`
}

func NewDefaultWorker() Worker {
	return Worker{
		SpotFleet: newDefaultSpotFleet(),
	}
}

func newDefaultSpotFleet() SpotFleet {
	return SpotFleet{
		SpotPrice:          "0.06",
		UnitRootVolumeSize: 30,
		RootVolumeType:     "gp2",
		LaunchSpecifications: []LaunchSpecification{
			NewLaunchSpecification(1, "t2.medium"),
			NewLaunchSpecification(2, "m3.large"),
			NewLaunchSpecification(2, "m4.large"),
		},
	}
}

func NewLaunchSpecification(weightedCapacity int, instanceType string) LaunchSpecification {
	return LaunchSpecification{
		WeightedCapacity: weightedCapacity,
		InstanceType:     instanceType,
		RootVolumeSize:   0,
		RootVolumeIOPS:   0,
		RootVolumeType:   "",
	}
}

func (c Worker) Valid() error {
	if err := c.SpotFleet.Valid(); err != nil {
		return err
	}

	return nil
}

func (c SpotFleet) Valid() error {
	for i, spec := range c.LaunchSpecifications {
		if err := spec.Valid(); err != nil {
			return fmt.Errorf("invalid launchSpecification at index %d: %v", i, err)
		}
	}
	return nil
}

func (c LaunchSpecification) Valid() error {
	if c.RootVolumeType == "io1" {
		if c.RootVolumeIOPS < 100 || c.RootVolumeIOPS > 2000 {
			return fmt.Errorf("invalid rootVolumeIOPS: %d", c.RootVolumeIOPS)
		}
	} else {
		if c.RootVolumeIOPS != 0 {
			return fmt.Errorf("invalid rootVolumeIOPS for volume type '%s': %d", c.RootVolumeType, c.RootVolumeIOPS)
		}

		if c.RootVolumeType != "standard" && c.RootVolumeType != "gp2" {
			return fmt.Errorf("invalid rootVolumeType: %s", c.RootVolumeType)
		}
	}
	return nil
}

func (f SpotFleet) Enabled() bool {
	return f.TargetCapacity > 0
}

func (f SpotFleet) IAMFleetRoleRef() string {
	if f.IAMFleetRoleARN == "" {
		return `{"Fn::Join":["", [ "arn:aws:iam::", {"Ref":"AWS::AccountId"}, ":role/aws-ec2-spot-fleet-role" ]]}`
	} else {
		return f.IAMFleetRoleARN
	}
}
