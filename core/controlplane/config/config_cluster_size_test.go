package config

import (
	"fmt"
	"strings"
	"testing"
)

const zeroOrGreaterError = "must be zero or greater"
const disjointConfigError = "can only be specified without"
const lessThanOrEqualError = "must be less than or equal to"

func TestASGsAllDefaults(t *testing.T) {
	checkControllerASGs(nil, nil, nil, nil, 1, 1, 0, "", t)
}

func TestASGsDefaultToMainCount(t *testing.T) {
	configuredCount := 6
	checkControllerASGs(&configuredCount, nil, nil, nil, 6, 6, 5, "", t)
}

func TestASGsInvalidMainCount(t *testing.T) {
	configuredCount := -1
	checkControllerASGs(&configuredCount, nil, nil, nil, 0, 0, 0, zeroOrGreaterError, t)
}

func TestASGsOnlyMinConfigured(t *testing.T) {
	configuredMin := 4
	// we expect min cannot be configured without a max
	checkControllerASGs(nil, &configuredMin, nil, nil, 0, 0, 0, lessThanOrEqualError, t)
}

func TestASGsOnlyMaxConfigured(t *testing.T) {
	configuredMax := 3
	// we expect min to be equal to main count if only max specified
	checkControllerASGs(nil, nil, &configuredMax, nil, 1, 3, 2, "", t)
}

func TestASGsMinMaxConfigured(t *testing.T) {
	configuredMin := 2
	configuredMax := 5
	checkControllerASGs(nil, &configuredMin, &configuredMax, nil, 2, 5, 4, "", t)
}

func TestASGsInvalidMin(t *testing.T) {
	configuredMin := -1
	configuredMax := 5
	checkControllerASGs(nil, &configuredMin, &configuredMax, nil, 0, 0, 0, zeroOrGreaterError, t)
}

func TestASGsInvalidMax(t *testing.T) {
	configuredMin := 1
	configuredMax := -1
	checkControllerASGs(nil, &configuredMin, &configuredMax, nil, 0, 0, 0, zeroOrGreaterError, t)
}

func TestASGsMinConfiguredWithMainCount(t *testing.T) {
	configuredCount := 2
	configuredMin := 4
	checkControllerASGs(&configuredCount, &configuredMin, nil, nil, 0, 0, 0, disjointConfigError, t)
}

func TestASGsMaxConfiguredWithMainCount(t *testing.T) {
	configuredCount := 2
	configuredMax := 4
	checkControllerASGs(&configuredCount, nil, &configuredMax, nil, 0, 0, 0, disjointConfigError, t)
}

func TestASGsMinMaxConfiguredWithMainCount(t *testing.T) {
	configuredCount := 2
	configuredMin := 3
	configuredMax := 4
	checkControllerASGs(&configuredCount, &configuredMin, &configuredMax, nil, 0, 0, 0, disjointConfigError, t)
}

func TestASGsMinInServiceConfigured(t *testing.T) {
	configuredMin := 5
	configuredMax := 10
	configuredMinInService := 7
	checkControllerASGs(nil, &configuredMin, &configuredMax, &configuredMinInService, 5, 10, 7, "", t)
}

const testConfig = minimalConfigYaml + `
subnets:
  - availabilityZone: ap-northeast-1a
    instanceCIDR: 10.0.1.0/24
  - availabilityZone: ap-northeast-1c
    instanceCIDR: 10.0.2.0/24
`

func checkControllerASGs(configuredCount *int, configuredMin *int, configuredMax *int, configuredMinInstances *int,
	expectedMin int, expectedMax int, expectedMinInstances int, expectedError string, t *testing.T) {
	checkControllerASG(configuredCount, configuredMin, configuredMax, configuredMinInstances,
		expectedMin, expectedMax, expectedMinInstances, expectedError, t)
}

func checkControllerASG(configuredCount *int, configuredMin *int, configuredMax *int, configuredMinInstances *int,
	expectedMin int, expectedMax int, expectedMinInstances int, expectedError string, t *testing.T) {
	config := testConfig

	countConfig := ""
	if configuredCount != nil {
		countConfig = fmt.Sprintf("  count: %d\n", *configuredCount)
	}
	asgConfig := buildASGConfig(configuredMin, configuredMax, configuredMinInstances)
	concatConfig := countConfig + asgConfig
	if concatConfig != "" {
		// empty `controller` traps go-yaml to override the whole Controller with a zero-value, which is not what we expect
		config += "controller:\n" + concatConfig
	}

	cluster, err := ClusterFromBytes([]byte(config))
	if err != nil {
		if expectedError == "" || !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Failed to validate cluster with controller config %s: %v", config, err)
		}
	} else {
		if expectedError != "" {
			t.Errorf("expected error \"%s\" not occurred", expectedError)
			t.FailNow()
		}

		config, err := cluster.Config()
		if err != nil {
			t.Errorf("Failed to create cluster config: %v", err)
		} else {
			if config.MinControllerCount() != expectedMin {
				t.Errorf("Controller ASG min count did not match the expected value: actual value of %d != expected value of %d",
					config.MinControllerCount(), expectedMin)
			}
			if config.MaxControllerCount() != expectedMax {
				t.Errorf("Controller ASG max count did not match the expected value: actual value of %d != expected value of %d",
					config.MaxControllerCount(), expectedMax)
			}
			if config.ControllerRollingUpdateMinInstancesInService() != expectedMinInstances {
				t.Errorf("Controller ASG rolling update min instances count did not match the expected value: actual value of %d != expected value of %d",
					config.ControllerRollingUpdateMinInstancesInService(), expectedMinInstances)
			}
		}
	}
}

func buildASGConfig(configuredMin *int, configuredMax *int, configuredMinInstances *int) string {
	asg := ""
	if configuredMin != nil {
		asg += fmt.Sprintf("    minSize: %d\n", *configuredMin)
	}
	if configuredMax != nil {
		asg += fmt.Sprintf("    maxSize: %d\n", *configuredMax)
	}
	if configuredMinInstances != nil {
		asg += fmt.Sprintf("    rollingUpdateMinInstancesInService: %d\n", *configuredMinInstances)
	}
	if asg != "" {
		return "  autoScalingGroup:\n" + asg
	}
	return ""
}
