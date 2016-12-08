package config

import (
	"strings"
	"testing"
)

func TestASGsAllDefaults(t *testing.T) {
	checkControllerASG(nil, nil, nil, nil, 1, 1, 0, "", t)
	checkWorkerASG(nil, nil, nil, nil, 1, 1, 0, "", t)
}

func TestASGsDefaultToMainCount(t *testing.T) {
	configuredCount := 6
	checkControllerASG(&configuredCount, nil, nil, nil, 6, 6, 5, "", t)
	checkWorkerASG(&configuredCount, nil, nil, nil, 6, 6, 5, "", t)
}

func TestASGsMinConfigured(t *testing.T) {
	configuredMin := 4
	// we expect max to be equal min if only min specified
	checkControllerASG(nil, &configuredMin, nil, nil, 4, 4, 3, "", t)
	checkWorkerASG(nil, &configuredMin, nil, nil, 4, 4, 3, "", t)
}

func TestASGsMaxConfigured(t *testing.T) {
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

func TestASGsMinConfiguredWithMainCount(t *testing.T) {
	configuredCount := 2
	configuredMin := 4
	checkControllerASG(&configuredCount, &configuredMin, nil, nil, 0, 0, 0, "controllerASG", t)
	checkWorkerASG(&configuredCount, &configuredMin, nil, nil, 0, 0, 0, "workerASG", t)
}

func TestASGsMaxConfiguredWithMainCount(t *testing.T) {
	configuredCount := 2
	configuredMax := 4
	checkControllerASG(&configuredCount, nil, &configuredMax, nil, 0, 0, 0, "controllerASG", t)
	checkWorkerASG(&configuredCount, nil, &configuredMax, nil, 0, 0, 0, "workerASG", t)
}

func TestASGsMinMaxConfiguredWithMainCount(t *testing.T) {
	configuredCount := 2
	configuredMin := 3
	configuredMax := 4
	checkControllerASG(&configuredCount, &configuredMin, &configuredMax, nil, 0, 0, 0, "controllerASG", t)
	checkWorkerASG(&configuredCount, &configuredMin, &configuredMax, nil, 0, 0, 0, "workerASG", t)
}

func TestASGsMinInServiceConfigured(t *testing.T) {
	configuredMin := 5
	configuredMax := 10
	configuredMinInService := 7
	checkControllerASG(nil, &configuredMin, &configuredMax, &configuredMinInService, 5, 10, 7, "", t)
	checkWorkerASG(nil, &configuredMin, &configuredMax, &configuredMinInService, 5, 10, 7, "", t)
}

func checkControllerASG(configuredCount *int, configuredMin *int, configuredMax *int, configuredMinInstances *int,
	expectedMin int, expectedMax int, expectedMinInstances int, expectedError string, t *testing.T) {
	cluster := NewDefaultCluster()

	// minimum setup to get the cluster config running in a test
	cluster.AmiId = "fake"
	cluster.EtcdCount = 0

	if configuredCount != nil {
		cluster.ControllerCount = *configuredCount
	}
	if configuredMin != nil {
		cluster.ControllerASG.MinSize = *configuredMin
	}
	if configuredMax != nil {
		cluster.ControllerASG.MaxSize = *configuredMax
	}
	if configuredMinInstances != nil {
		cluster.ControllerASG.RollingUpdateMinInstancesInService = *configuredMinInstances
	}

	config, err := cluster.Config()
	if err != nil {
		if expectedError == "" || !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Failed to create cluster config: %v", err)
		}
	} else {
		if config.ControllerASG.MinSize != expectedMin {
			t.Errorf("Controller ASG min count did not match the expected value: actual value of %d != expected value of %d",
				config.ControllerASG.MinSize, expectedMin)
		}
		if config.ControllerASG.MaxSize != expectedMax {
			t.Errorf("Controller ASG max count did not match the expected value: actual value of %d != expected value of %d",
				config.ControllerASG.MaxSize, expectedMax)
		}
		if config.ControllerASG.RollingUpdateMinInstancesInService != expectedMinInstances {
			t.Errorf("Controller ASG rolling update min instances count did not match the expected value: actual value of %d != expected value of %d",
				config.ControllerASG.RollingUpdateMinInstancesInService, expectedMinInstances)
		}
	}
}

func checkWorkerASG(configuredCount *int, configuredMin *int, configuredMax *int, configuredMinInstances *int,
	expectedMin int, expectedMax int, expectedMinInstances int, expectedError string, t *testing.T) {
	cluster := NewDefaultCluster()

	// minimum setup to get the cluster config running in a test
	cluster.AmiId = "fake"
	cluster.EtcdCount = 0

	if configuredCount != nil {
		cluster.WorkerCount = *configuredCount
	}
	if configuredMin != nil {
		cluster.WorkerASG.MinSize = *configuredMin
	}
	if configuredMax != nil {
		cluster.WorkerASG.MaxSize = *configuredMax
	}
	if configuredMinInstances != nil {
		cluster.WorkerASG.RollingUpdateMinInstancesInService = *configuredMinInstances
	}

	config, err := cluster.Config()
	if err != nil {
		if expectedError == "" || !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Failed to create cluster config: %v", err)
		}
	} else {
		if config.WorkerASG.MinSize != expectedMin {
			t.Errorf("Worker ASG min count did not match the expected value: actual value of %d != expected value of %d",
				config.WorkerASG.MinSize, expectedMin)
		}
		if config.WorkerASG.MaxSize != expectedMax {
			t.Errorf("Worker ASG max count did not match the expected value: actual value of %d != expected value of %d",
				config.WorkerASG.MaxSize, expectedMax)
		}
		if config.WorkerASG.RollingUpdateMinInstancesInService != expectedMinInstances {
			t.Errorf("Worker ASG rolling update min instances count did not match the expected value: actual value of %d != expected value of %d",
				config.WorkerASG.RollingUpdateMinInstancesInService, expectedMinInstances)
		}
	}
}
