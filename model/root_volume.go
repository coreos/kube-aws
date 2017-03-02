package model

import "fmt"

type RootVolume struct {
	RootVolumeType string `yaml:"rootVolumeType,omitempty"`
	RootVolumeIOPS int    `yaml:"rootVolumeIOPS,omitempty"`
	RootVolumeSize int    `yaml:"rootVolumeSize,omitempty"`
}

func NewGp2RootVolume(size int) RootVolume {
	return RootVolume{
		RootVolumeSize: size,
		RootVolumeIOPS: 0,
		RootVolumeType: "gp2",
	}
}

func NewIo1RootVolume(size int, iops int) RootVolume {
	return RootVolume{
		RootVolumeSize: size,
		RootVolumeIOPS: iops,
		RootVolumeType: "io1",
	}
}

func (v RootVolume) Validate() error {
	if v.RootVolumeType == "io1" {
		if v.RootVolumeIOPS < 100 || v.RootVolumeIOPS > 2000 {
			return fmt.Errorf(`invalid rootVolumeIOPS %d in %+v: rootVolumeIOPS must be between 100 and 2000`, v.RootVolumeIOPS, v)
		}
	} else {
		if v.RootVolumeIOPS != 0 {
			return fmt.Errorf(`invalid rootVolumeIOPS %d for volume type "%s" in %+v": rootVolumeIOPS must be 0 when rootVolumeType is "standard" or "gp2"`, v.RootVolumeIOPS, v.RootVolumeType, v)
		}

		if v.RootVolumeType != "standard" && v.RootVolumeType != "gp2" {
			return fmt.Errorf(`invalid rootVolumeType "%s" in %+v: rootVolumeType must be one of "standard", "gp2", "io1"`, v.RootVolumeType, v)
		}
	}
	return nil
}
