package api

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestValidateAsgSizes(t *testing.T) {
	var minSize int
	var rolMinInst int
	a := AutoScalingGroup{
		MinSize:                            &minSize,
		RollingUpdateMinInstancesInService: &rolMinInst,
	}

	err := a.Validate()
	require.NoError(t, err)

	// Expect error if minSize is negative
	minSize = -1
	err = a.Validate()
	require.EqualError(t, err, "`autoScalingGroup.minSize` must be zero or greater if specified")
	minSize = 1

	// Expect error if maxSize is negative
	a.MaxSize = -1
	err = a.Validate()
	require.EqualError(t, err, "`autoScalingGroup.maxSize` must be zero or greater if specified")
	a.MaxSize = 3

	// Expect error is minSize > maxSize
	minSize = 5
	err = a.Validate()
	require.EqualError(t, err, "`autoScalingGroup.minSize` (5) must be less than or equal to `autoScalingGroup.maxSize` (3), if you have specified only minSize please specify maxSize as well")
	minSize = 1

	// Expect error if rollingUpdate is negative
	rolMinInst = -1
	err = a.Validate()
	require.EqualError(t, err, "`autoScalingGroup.rollingUpdateMinInstancesInService` must be greater than or equal to 0 but was -1")
	rolMinInst = 1
}

func TestValidateAsgMixedInstances(t *testing.T) {
	a := AutoScalingGroup{
		MixedInstances: MixedInstances{
			Enabled: false,
		},
	}

	// Expect no error if MixedInstances are not enabled
	err := a.Validate()
	require.NoError(t, err)

	// Expect no error if mixed instances are enabled with default values
	a.MixedInstances.Enabled = true
	err = a.Validate()
	require.NoError(t, err)

	// Expect error if string fields set to incorrect values
	a.MixedInstances.OnDemandAllocationStrategy = "invalid-value"
	err = a.Validate()
	require.EqualError(t, err, "`mixedInstances.onDemandAllocationStrategy` must be equal to 'prioritized' if specified")
	a.MixedInstances.OnDemandAllocationStrategy = "prioritized"
	a.MixedInstances.SpotAllocationStrategy = "invalid-value"
	err = a.Validate()
	require.EqualError(t, err, "`mixedInstances.spotAllocationStrategy` must be equal to 'lowest-price' if specified")

	// Expect no error if string fields set to correct values
	a.MixedInstances.SpotAllocationStrategy = "lowest-price"
	err = a.Validate()
	require.NoError(t, err)

	// Testing invalid values (out of range) for some fields
	a.MixedInstances.OnDemandBaseCapacity = -1
	err = a.Validate()
	require.EqualError(t, err, "`mixedInstances.onDemandBaseCapacity` (-1) must be zero or greater if specified")
	a.MixedInstances.OnDemandBaseCapacity = 10

	a.MixedInstances.OnDemandPercentageAboveBaseCapacity = 102
	err = a.Validate()
	require.EqualError(t, err, "`mixedInstances.onDemandPercentageAboveBaseCapacity` (102) must be in range 0-100")

	a.MixedInstances.OnDemandPercentageAboveBaseCapacity = -1
	err = a.Validate()
	require.EqualError(t, err, "`mixedInstances.onDemandPercentageAboveBaseCapacity` (-1) must be in range 0-100")

	// Within ranges, everything should be fine again
	a.MixedInstances.OnDemandPercentageAboveBaseCapacity = 42
	a.MixedInstances.SpotInstancePools = 10
	err = a.Validate()
	require.NoError(t, err)

	// More out of range
	a.MixedInstances.SpotInstancePools = 42
	err = a.Validate()
	require.EqualError(t, err, "`mixedInstances.spotInstancePools` (42) must be in range 0-20")

	a.MixedInstances.SpotInstancePools = -1
	err = a.Validate()
	require.EqualError(t, err, "`mixedInstances.spotInstancePools` (-1) must be in range 0-20")
	a.MixedInstances.SpotInstancePools = 10

	// Last error: too long string for SpotPrice
	a.MixedInstances.SpotMaxPrice = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	err = a.Validate()
	require.EqualError(t, err, "`mixedInstances.spotMaxPrice` can have a maximum length of 255")

	// Every field filled with a valid value, no error expected
	a.MixedInstances.SpotMaxPrice = "2"
	err = a.Validate()
	require.NoError(t, err)
}
