package api

import (
	"fmt"
)

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
	UnknownKeys          `yaml:",inline"`
}

func (f SpotFleet) Enabled() bool {
	return f.TargetCapacity > 0
}

func (c SpotFleet) Validate() error {
	for i, spec := range c.LaunchSpecifications {
		if err := spec.Validate(); err != nil {
			return fmt.Errorf("invalid launchSpecification at index %d: %v", i, err)
		}
	}

	return nil
}

func (f *SpotFleet) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type t SpotFleet
	work := t(newDefaultSpotFleet())
	if err := unmarshal(&work); err != nil {
		return fmt.Errorf("failed to parse node pool config: %v", err)
	}
	*f = SpotFleet(work)

	launchSpecs := []LaunchSpecification{}
	for _, spec := range f.LaunchSpecifications {
		if spec.RootVolume.Type == "" {
			spec.RootVolume.Type = f.RootVolumeType
		}
		if spec.RootVolume.Size == 0 {
			spec.RootVolume.Size = f.UnitRootVolumeSize * spec.WeightedCapacity
		}
		if spec.RootVolume.Type == "io1" && spec.RootVolume.IOPS == 0 {
			spec.RootVolume.IOPS = f.UnitRootVolumeIOPS * spec.WeightedCapacity
		}
		launchSpecs = append(launchSpecs, spec)
	}
	f.LaunchSpecifications = launchSpecs

	return nil
}

func (f SpotFleet) IAMFleetRoleRef() string {
	if f.IAMFleetRoleARN == "" {
		return `{"Fn::Join":["", [ "arn:aws:iam::", {"Ref":"AWS::AccountId"}, ":role/aws-ec2-spot-fleet-tagging-role" ]]}`
	} else {
		return fmt.Sprintf(`"%s"`, f.IAMFleetRoleARN)
	}
}
