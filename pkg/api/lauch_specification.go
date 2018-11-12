package api

import (
	"fmt"
)

type LaunchSpecification struct {
	WeightedCapacity int    `yaml:"weightedCapacity,omitempty"`
	InstanceType     string `yaml:"instanceType,omitempty"`
	SpotPrice        string `yaml:"spotPrice,omitempty"`
	RootVolume       `yaml:"rootVolume,omitempty"`
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

	return nil
}

func (c LaunchSpecification) Validate() error {
	if err := c.RootVolume.Validate(); err != nil {
		return err
	}

	return nil
}
