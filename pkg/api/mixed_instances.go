package api

import "fmt"

type MixedInstances struct {
	Enabled                             bool     `yaml:"enabled,omitempty"`
	OnDemandAllocationStrategy          string   `yaml:"onDemandAllocationStrategy,omitempty"`
	OnDemandBaseCapacity                int      `yaml:"onDemandBaseCapacity,omitempty"`
	OnDemandPercentageAboveBaseCapacity int      `yaml:"onDemandPercentageAboveBaseCapacity,omitempty"`
	SpotAllocationStrategy              string   `yaml:"spotAllocationStrategy,omitempty"`
	SpotInstancePools                   int      `yaml:"spotInstancePools,omitempty"`
	SpotMaxPrice                        string   `yaml:"spotMaxPrice,omitempty"`
	InstanceTypes                       []string `yaml:"instanceTypes,omitempty"`
}

func (mi MixedInstances) Validate() error {
	// See https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-autoscaling-autoscalinggroup-instancesdistribution.html for valid values
	if mi.OnDemandAllocationStrategy != "" && mi.OnDemandAllocationStrategy != "prioritized" {
		return fmt.Errorf("`mixedInstances.onDemandAllocationStrategy` must be equal to 'prioritized' if specified")
	}
	if mi.OnDemandBaseCapacity < 0 {
		return fmt.Errorf("`mixedInstances.onDemandBaseCapacity` (%d) must be zero or greater if specified", mi.OnDemandBaseCapacity)
	}
	if mi.OnDemandPercentageAboveBaseCapacity < 0 || mi.OnDemandPercentageAboveBaseCapacity > 100 {
		return fmt.Errorf("`mixedInstances.onDemandPercentageAboveBaseCapacity` (%d) must be in range 0-100", mi.OnDemandPercentageAboveBaseCapacity)
	}
	if mi.SpotAllocationStrategy != "" && mi.SpotAllocationStrategy != "lowest-price" {
		return fmt.Errorf("`mixedInstances.spotAllocationStrategy` must be equal to 'lowest-price' if specified")
	}
	if mi.SpotInstancePools < 0 || mi.SpotInstancePools > 20 {
		return fmt.Errorf("`mixedInstances.spotInstancePools` (%d) must be in range 0-20", mi.SpotInstancePools)
	}
	if len(mi.SpotMaxPrice) > 255 {
		return fmt.Errorf("`mixedInstances.spotMaxPrice` can have a maximum length of 255")
	}

	return nil
}
