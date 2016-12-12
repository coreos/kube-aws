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
	checkControllerASG(nil, nil, nil, nil, 1, 1, 0, "", t)
	checkWorkerASG(nil, nil, nil, nil, 1, 1, 0, "", t)
}

func TestASGsDefaultToMainCount(t *testing.T) {
	configuredCount := 6
	checkControllerASG(&configuredCount, nil, nil, nil, 6, 6, 5, "", t)
	checkWorkerASG(&configuredCount, nil, nil, nil, 6, 6, 5, "", t)
}

func TestASGsInvalidMainCount(t *testing.T) {
	configuredCount := -1
	checkControllerASG(&configuredCount, nil, nil, nil, 0, 0, 0, zeroOrGreaterError, t)
	checkWorkerASG(&configuredCount, nil, nil, nil, 0, 0, 0, zeroOrGreaterError, t)
}

func TestASGsOnlyMinConfigured(t *testing.T) {
	configuredMin := 4
	// we expect min cannot be configured without a max
	checkControllerASG(nil, &configuredMin, nil, nil, 0, 0, 0, lessThanOrEqualError, t)
	checkWorkerASG(nil, &configuredMin, nil, nil, 0, 0, 0, lessThanOrEqualError, t)
}

func TestASGsOnlyMaxConfigured(t *testing.T) {
	configuredMax := 3
	// we expect min to be equal to main count if only max specified
	checkControllerASG(nil, nil, &configuredMax, nil, 1, 3, 2, "", t)
	checkWorkerASG(nil, nil, &configuredMax, nil, 1, 3, 2, "", t)
}

func TestASGsMinMaxConfigured(t *testing.T) {
	configuredMin := 2
	configuredMax := 5
	checkControllerASG(nil, &configuredMin, &configuredMax, nil, 2, 5, 4, "", t)
	checkWorkerASG(nil, &configuredMin, &configuredMax, nil, 2, 5, 4, "", t)
}

func TestASGsInvalidMin(t *testing.T) {
	configuredMin := -1
	configuredMax := 5
	checkControllerASG(nil, &configuredMin, &configuredMax, nil, 0, 0, 0, zeroOrGreaterError, t)
	checkWorkerASG(nil, &configuredMin, &configuredMax, nil, 0, 0, 0, zeroOrGreaterError, t)
}

func TestASGsInvalidMax(t *testing.T) {
	configuredMin := 1
	configuredMax := -1
	checkControllerASG(nil, &configuredMin, &configuredMax, nil, 0, 0, 0, zeroOrGreaterError, t)
	checkWorkerASG(nil, &configuredMin, &configuredMax, nil, 0, 0, 0, zeroOrGreaterError, t)
}

func TestASGsMinConfiguredWithMainCount(t *testing.T) {
	configuredCount := 2
	configuredMin := 4
	checkControllerASG(&configuredCount, &configuredMin, nil, nil, 0, 0, 0, disjointConfigError, t)
	checkWorkerASG(&configuredCount, &configuredMin, nil, nil, 0, 0, 0, disjointConfigError, t)
}

func TestASGsMaxConfiguredWithMainCount(t *testing.T) {
	configuredCount := 2
	configuredMax := 4
	checkControllerASG(&configuredCount, nil, &configuredMax, nil, 0, 0, 0, disjointConfigError, t)
	checkWorkerASG(&configuredCount, nil, &configuredMax, nil, 0, 0, 0, disjointConfigError, t)
}

func TestASGsMinMaxConfiguredWithMainCount(t *testing.T) {
	configuredCount := 2
	configuredMin := 3
	configuredMax := 4
	checkControllerASG(&configuredCount, &configuredMin, &configuredMax, nil, 0, 0, 0, disjointConfigError, t)
	checkWorkerASG(&configuredCount, &configuredMin, &configuredMax, nil, 0, 0, 0, disjointConfigError, t)
}

func TestASGsMinInServiceConfigured(t *testing.T) {
	configuredMin := 5
	configuredMax := 10
	configuredMinInService := 7
	checkControllerASG(nil, &configuredMin, &configuredMax, &configuredMinInService, 5, 10, 7, "", t)
	checkWorkerASG(nil, &configuredMin, &configuredMax, &configuredMinInService, 5, 10, 7, "", t)
}

const testConfig = minimalConfigYaml + `
subnets:
  - availabilityZone: ap-northeast-1a
    instanceCIDR: 10.0.1.0/24
  - availabilityZone: ap-northeast-1c
    instanceCIDR: 10.0.2.0/24
`

func checkControllerASG(configuredCount *int, configuredMin *int, configuredMax *int, configuredMinInstances *int,
	expectedMin int, expectedMax int, expectedMinInstances int, expectedError string, t *testing.T) {
	config := testConfig
	if configuredCount != nil {
		config += fmt.Sprintf("controllerCount: %d\n", *configuredCount)
	}
	config += "controller:\n" + buildASGConfig(configuredMin, configuredMax, configuredMinInstances)

	cluster, err := ClusterFromBytes([]byte(config))
	if err != nil {
		if expectedError == "" || !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Failed to validate cluster with controller config: %v", err)
		}
	} else {
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

func checkWorkerASG(configuredCount *int, configuredMin *int, configuredMax *int, configuredMinInstances *int,
	expectedMin int, expectedMax int, expectedMinInstances int, expectedError string, t *testing.T) {
	config := testConfig
	if configuredCount != nil {
		config += fmt.Sprintf("workerCount: %d\n", *configuredCount)
	}
	config += "worker:\n" + buildASGConfig(configuredMin, configuredMax, configuredMinInstances)

	cluster, err := ClusterFromBytes([]byte(config))
	if err != nil {
		if expectedError == "" || !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Failed to validate cluster with worker config: %v", err)
		}
	} else {
		config, err := cluster.Config()
		if err != nil {
			if expectedError == "" || !strings.Contains(err.Error(), expectedError) {
				t.Errorf("Failed to create cluster config: %v", err)
			}
		} else {
			if config.MinWorkerCount() != expectedMin {
				t.Errorf("Worker ASG min count did not match the expected value: actual value of %d != expected value of %d",
					config.MinWorkerCount(), expectedMin)
			}
			if config.MaxWorkerCount() != expectedMax {
				t.Errorf("Worker ASG max count did not match the expected value: actual value of %d != expected value of %d",
					config.MaxWorkerCount(), expectedMax)
			}
			if config.WorkerRollingUpdateMinInstancesInService() != expectedMinInstances {
				t.Errorf("Worker ASG rolling update min instances count did not match the expected value: actual value of %d != expected value of %d",
					config.WorkerRollingUpdateMinInstancesInService(), expectedMinInstances)
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
