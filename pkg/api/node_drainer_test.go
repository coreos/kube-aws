package api

import (
	"testing"
)

func TestDrainTimeoutInSeconds(t *testing.T) {
	testCases := []struct {
		timeoutInMins int
		timeoutInSecs int
	}{
		{
			timeoutInMins: 0,
			timeoutInSecs: 0,
		},
		{
			timeoutInMins: 1,
			timeoutInSecs: 60,
		},
		{
			timeoutInMins: 2,
			timeoutInSecs: 120,
		},
	}

	for _, testCase := range testCases {
		drainer := NodeDrainer{
			DrainTimeout: testCase.timeoutInMins,
		}
		actual := drainer.DrainTimeoutInSeconds()
		if actual != testCase.timeoutInSecs {
			t.Errorf("Expected drain timeout in secs to be %d, but was %d", testCase.timeoutInSecs, actual)
		}
	}
}

func TestValidate(t *testing.T) {
	testCases := []struct {
		enabled                   bool
		drainTimeout              int
		isValid                   bool
		unschedulableWhenCordoned bool
	}{
		// Invalid, drainTimeout is < 1
		{
			enabled:                   true,
			drainTimeout:              0,
			unschedulableWhenCordoned: true,
			isValid:                   false,
		},

		// Invalid, drainTimeout > 60
		{
			enabled:                   true,
			drainTimeout:              61,
			unschedulableWhenCordoned: true,
			isValid:                   false,
		},

		// Valid, disabled
		{
			enabled:                   false,
			drainTimeout:              0,
			unschedulableWhenCordoned: true,
			isValid:                   true,
		},

		// Valid, timeout within boundaries
		{
			enabled:                   true,
			drainTimeout:              1,
			unschedulableWhenCordoned: true,
			isValid:                   true,
		},

		// Valid, timeout within boundaries
		{
			enabled:                   true,
			drainTimeout:              60,
			unschedulableWhenCordoned: true,
			isValid:                   true,
		},
	}

	for _, testCase := range testCases {
		drainer := NodeDrainer{
			Enabled:                   testCase.enabled,
			DrainTimeout:              testCase.drainTimeout,
			UnschedulableWhenCordoned: testCase.unschedulableWhenCordoned,
		}

		err := drainer.Validate()
		if testCase.isValid && err != nil {
			t.Errorf("Expected node drainer to be valid, but it was not: %v", err)
		}

		if !testCase.isValid && err == nil {
			t.Errorf("Expected node drainer to be invalid, but it was not")
		}
	}
}
