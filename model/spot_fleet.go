package model

import (
	"fmt"
	"strconv"
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
}

func (f SpotFleet) Enabled() bool {
	return f.TargetCapacity > 0
}

func (c SpotFleet) Valid() error {
	for i, spec := range c.LaunchSpecifications {
		if err := spec.Valid(); err != nil {
			return fmt.Errorf("invalid launchSpecification at index %d: %v", i, err)
		}
	}

	return nil
}

func (f SpotFleet) WithDefaults() SpotFleet {
	defaults := newDefaultSpotFleet()

	if f.SpotPrice == "" {
		f.SpotPrice = defaults.SpotPrice
	}

	if f.UnitRootVolumeSize == 0 {
		f.UnitRootVolumeSize = defaults.UnitRootVolumeSize
	}

	if f.UnitRootVolumeIOPS == 0 {
		f.UnitRootVolumeIOPS = defaults.UnitRootVolumeIOPS
	}

	if f.RootVolumeType == "" {
		f.RootVolumeType = defaults.RootVolumeType
	}

	if len(f.LaunchSpecifications) == 0 {
		f.LaunchSpecifications = defaults.LaunchSpecifications
	}

	launchSpecs := []LaunchSpecification{}
	for _, spec := range f.LaunchSpecifications {
		if spec.SpotPrice == "" {
			p, err := strconv.ParseFloat(f.SpotPrice, 64)
			if err != nil {
				panic(fmt.Errorf(`failed to parse float from spotPrice "%s" in %+v: %v`, f.SpotPrice, f, err))
			}
			spec.SpotPrice = strconv.FormatFloat(p*float64(spec.WeightedCapacity), 'f', -1, 64)
		}
		if spec.RootVolumeType == "" {
			spec.RootVolumeType = f.RootVolumeType
		}
		if spec.RootVolumeSize == 0 {
			spec.RootVolumeSize = f.UnitRootVolumeSize * spec.WeightedCapacity
		}
		if spec.RootVolumeType == "io1" && spec.RootVolumeIOPS == 0 {
			spec.RootVolumeIOPS = f.UnitRootVolumeIOPS * spec.WeightedCapacity
		}
		launchSpecs = append(launchSpecs, spec)
	}
	f.LaunchSpecifications = launchSpecs

	return f
}

func (f SpotFleet) IAMFleetRoleRef() string {
	if f.IAMFleetRoleARN == "" {
		return `{"Fn::Join":["", [ "arn:aws:iam::", {"Ref":"AWS::AccountId"}, ":role/aws-ec2-spot-fleet-role" ]]}`
	} else {
		return f.IAMFleetRoleARN
	}
}
