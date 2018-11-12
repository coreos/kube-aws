package api

import (
	"fmt"
	"regexp"
	"strings"
)

type NodeVolumeMount struct {
	Type       string `yaml:"type,omitempty"`
	Iops       int    `yaml:"iops,omitempty"`
	Size       int    `yaml:"size,omitempty"`
	Device     string `yaml:"device,omitempty"`
	Filesystem string `yaml:"filesystem,omitempty"`
	Path       string `yaml:"path,omitempty"`
	CreateTmp  bool   `yaml:"createTmp,omitempty"`
}

func (v NodeVolumeMount) SystemdMountName() string {
	return strings.Replace(strings.TrimLeft(v.Path, "/"), "/", "-", -1)
}

func (v NodeVolumeMount) FilesystemType() string {
	if v.Filesystem == "" {
		return "xfs"
	}
	return v.Filesystem
}

func (v NodeVolumeMount) Validate() error {
	if v.Type == "io1" {
		if v.Iops < 100 || v.Iops > 20000 {
			return fmt.Errorf(`invalid iops "%d" in %+v: iops must be between "100" and "20000"`, v.Iops, v)
		}
	} else {
		if v.Iops != 0 {
			return fmt.Errorf(`invalid iops "%d" for volume type "%s" in %+v: iops must be "0" when type is "standard" or "gp2"`, v.Iops, v.Type, v)
		}

		if v.Type != "standard" && v.Type != "gp2" {
			return fmt.Errorf(`invalid type "%s" in %+v: type must be one of "standard", "gp2", "io1"`, v.Type, v)
		}
	}
	if v.Filesystem != "" {
		if v.Filesystem != "xfs" && v.Filesystem != "ext4" {
			return fmt.Errorf(`invalid filesystem type "%s" in %+v: type must be one of "xfs", "ext4"`, v.Type, v)
		}
	}

	if v.Size <= 0 {
		return fmt.Errorf(`invalid size "%d" in %+v: size must be greater than "0"`, v.Size, v)
	}

	if v.Path == "" {
		return fmt.Errorf(`invalid path "%s" in %v: path cannot be empty`, v.Path, v)
	} else if regexp.MustCompile("^[a-zA-Z0-9/]*$").MatchString(v.Path) != true || strings.HasSuffix(v.Path, "/") || strings.HasPrefix(v.Path, "/") == false || strings.Contains(v.Path, "//") {
		return fmt.Errorf(`invalid path "%s" in %+v`, v.Path, v)
	}

	if strings.Compare(v.Device, "/dev/xvdf") == -1 || strings.Compare(v.Device, "/dev/xvdz") == 1 {
		return fmt.Errorf(`invalid device "%s" in %+v: device must be a value from "/dev/xvdf" to "/dev/xvdz"`, v.Device, v)
	}

	return nil
}

func ValidateVolumeMounts(volumes []NodeVolumeMount) error {
	paths := make(map[string]bool)
	devices := make(map[string]bool)
	for _, volume := range volumes {
		if err := volume.Validate(); err != nil {
			return err
		}
		if paths[volume.Path] == true {
			return fmt.Errorf("duplicate volumeMount path detected (%s) - values must be unique", volume.Path)
		}
		paths[volume.Path] = true
		if devices[volume.Device] == true {
			return fmt.Errorf("duplicate volumeMount device detected (%s) - values must be unique", volume.Device)
		}
		devices[volume.Device] = true
	}
	return nil
}
