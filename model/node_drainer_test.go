package model

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

func TestDrainIntervalInSeconds(t *testing.T) {
	testCases := []struct {
		intervalInMins int
		intervalInSecs int
	}{
		{
			intervalInMins: 0,
			intervalInSecs: 0,
		},
		{
			intervalInMins: 1,
			intervalInSecs: 60,
		},
		{
			intervalInMins: 2,
			intervalInSecs: 120,
		},
	}

	for _, testCase := range testCases {
		drainer := NodeDrainer{
			DrainInterval: testCase.intervalInMins,
		}
		actual := drainer.DrainIntervalInSeconds()
		if actual != testCase.intervalInSecs {
			t.Errorf("Expected drain interval in secs to be %d, but was %d", testCase.intervalInSecs, actual)
		}
	}
}
