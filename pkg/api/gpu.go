package api

import (
	"errors"
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"strings"
)

var GPUEnabledInstanceFamily = []string{"p2", "p3", "g2", "g3"}

type Gpu struct {
	Nvidia NvidiaSetting `yaml:"nvidia"`
}

type NvidiaSetting struct {
	Enabled bool   `yaml:"enabled,omitempty"`
	Version string `yaml:"version,omitempty"`
}

func isGpuEnabledInstanceType(instanceType string) bool {
	for _, family := range GPUEnabledInstanceFamily {
		if strings.HasPrefix(instanceType, family) {
			return true
		}
	}
	return false
}

func newDefaultGpu() Gpu {
	return Gpu{
		Nvidia: NvidiaSetting{
			Enabled: false,
			Version: "",
		},
	}
}

// This function is used when rendering cloud-config-worker
func (c NvidiaSetting) IsEnabledOn(instanceType string) bool {
	return isGpuEnabledInstanceType(instanceType) && c.Enabled
}

func (c Gpu) Validate(instanceType string, experimentalGpuSupportEnabled bool) error {
	if c.Nvidia.Enabled && !isGpuEnabledInstanceType(instanceType) {
		return errors.New(fmt.Sprintf("instance type %v doesn't support GPU. You can enable Nvidia driver intallation support only when use %v instance family.", instanceType, GPUEnabledInstanceFamily))
	}
	if !c.Nvidia.Enabled && !experimentalGpuSupportEnabled && isGpuEnabledInstanceType(instanceType) {
		logger.Warnf("Nvidia GPU driver intallation is disabled although instance type %v does support GPU.  You have to install Nvidia GPU driver by yourself to schedule gpu resource.\n", instanceType)
	}
	if c.Nvidia.Enabled && experimentalGpuSupportEnabled {
		return errors.New(`Only one of gpu.nvidia.enabled and experimental.gpuSupport.enabled are allowed at one time.`)
	}
	if c.Nvidia.Enabled && len(c.Nvidia.Version) == 0 {
		return errors.New(`gpu.nvidia.version must not be empty when gpu.nvidia is enabled.`)
	}

	return nil
}
