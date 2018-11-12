package api

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/logger"
)

type RootVolume struct {
	Size        int    `yaml:"size,omitempty"`
	Type        string `yaml:"type,omitempty"`
	IOPS        int    `yaml:"iops,omitempty"`
	UnknownKeys `yaml:",inline"`
}

func NewGp2RootVolume(size int) RootVolume {
	return RootVolume{
		Size: size,
		IOPS: 0,
		Type: "gp2",
	}
}

func NewIo1RootVolume(size int, iops int) RootVolume {
	return RootVolume{
		Size: size,
		IOPS: iops,
		Type: "io1",
	}
}

func (v RootVolume) Validate() error {
	if v.Type == "io1" {
		if v.IOPS < 100 || v.IOPS > 20000 {
			return fmt.Errorf(`invalid rootVolumeIOPS %d in %+v: rootVolumeIOPS must be between 100 and 20000`, v.IOPS, v)
		}
	} else {
		if v.IOPS != 0 {
			return fmt.Errorf(`invalid rootVolumeIOPS %d for volume type "%s" in %+v": rootVolumeIOPS must be 0 when rootVolumeType is "standard" or "gp2"`, v.IOPS, v.Type, v)
		}

		if v.Type != "standard" && v.Type != "gp2" {
			return fmt.Errorf(`invalid rootVolumeType "%s" in %+v: rootVolumeType must be one of "standard", "gp2", "io1"`, v.Type, v)
		}
	}
	return nil
}

func (v RootVolume) RootVolumeIOPS() int {
	logger.Warn("RootVolumeIOPS is deprecated and will be removed in v0.9.7. Please use RootVolume.IOPS instead")
	return v.IOPS
}

func (v RootVolume) RootVolumeType() string {
	logger.Warn("RootVolumeType is deprecated and will be removed in v0.9.7. Please use RootVolume.Type instead")
	return v.Type
}

func (v RootVolume) RootVolumeSize() int {
	logger.Warn("RootVolumeSize is deprecated and will be removed in v0.9.7. Please use RootVolume.Size instead")
	return v.Size
}
