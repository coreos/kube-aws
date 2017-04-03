package model

import (
	"fmt"
)

type LaunchSpecification struct {
	WeightedCapacity     int    `yaml:"weightedCapacity,omitempty"`
	InstanceType         string `yaml:"instanceType,omitempty"`
	SpotPrice            string `yaml:"spotPrice,omitempty"`
	DeprecatedRootVolume `yaml:",inline"`
	RootVolume           `yaml:"rootVolume,omitempty"`
}

func NewLaunchSpecification(weightedCapacity int, instanceType string) LaunchSpecification {
	return LaunchSpecification{
		WeightedCapacity: weightedCapacity,
		InstanceType:     instanceType,
	}
}

func (s *LaunchSpecification) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type t LaunchSpecification
	work := t(LaunchSpecification{})
	if err := unmarshal(&work); err != nil {
		return fmt.Errorf("failed to parse node pool config: %v", err)
	}
	*s = LaunchSpecification(work)

	// TODO Remove deprecated keys in v0.9.7
	if s.DeprecatedRootVolumeIOPS != nil {
		fmt.Println("WARN: launchSpecifications[].rootVolumeIOPS is deprecated and will be removed in v0.9.7. Please use launchSpecifications[].rootVolume.iops instead")
		s.RootVolume.IOPS = *s.DeprecatedRootVolumeIOPS
	}
	if s.DeprecatedRootVolumeSize != nil {
		fmt.Println("WARN: launchSpecifications[].rootVolumeSize is deprecated and will be removed in v0.9.7. Please use launchSpecifications[].rootVolume.size instead")
		s.RootVolume.Size = *s.DeprecatedRootVolumeSize
	}
	if s.DeprecatedRootVolumeType != nil {
		fmt.Println("WARN: launchSpecifications[].rootVolumeType is deprecated and will be removed in v0.9.7. Please use launchSpecifications[].rootVolume.type instead")
		s.RootVolume.Type = *s.DeprecatedRootVolumeType
	}

	return nil
}

func (c LaunchSpecification) Valid() error {
	if err := c.RootVolume.Validate(); err != nil {
		return err
	}

	return nil
}
