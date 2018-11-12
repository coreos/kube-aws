package api

import "strings"

type EC2Instance struct {
	Count         int    `yaml:"count,omitempty"`
	CreateTimeout string `yaml:"createTimeout,omitempty"`
	InstanceType  string `yaml:"instanceType,omitempty"`
	RootVolume    `yaml:"rootVolume,omitempty"`
	Tenancy       string            `yaml:"tenancy,omitempty"`
	InstanceTags  map[string]string `yaml:"instanceTags,omitempty"`
}

var nvmeEC2InstanceFamily = []string{"c5", "m5"}

func isNvmeEC2InstanceType(instanceType string) bool {
	for _, family := range nvmeEC2InstanceFamily {
		if strings.HasPrefix(instanceType, family) {
			return true
		}
	}
	return false
}

// This function is used when rendering cloud-config-worker
func (e EC2Instance) HasNvmeDevices() bool {
	return isNvmeEC2InstanceType(e.InstanceType)
}
