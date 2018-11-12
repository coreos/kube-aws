package api

import (
	"fmt"
	"regexp"
	"strings"
)

type Raid0Mount struct {
	Type      string   `yaml:"type,omitempty"`
	Iops      int      `yaml:"iops,omitempty"`
	Size      int      `yaml:"size,omitempty"`
	Devices   []string `yaml:"devices,omitempty"`
	Path      string   `yaml:"path,omitempty"`
	CreateTmp bool     `yaml:"createTmp,omitempty"`
}

func (r Raid0Mount) SystemdMountName() string {
	return strings.Replace(strings.TrimLeft(r.Path, "/"), "/", "-", -1)
}

func (r Raid0Mount) DeviceList() string {
	return strings.Join(r.Devices, " ")
}

func (r Raid0Mount) NumDevices() int {
	return len(r.Devices)
}

func (r Raid0Mount) Validate() error {
	if r.Type == "io1" {
		if r.Iops < 100 || r.Iops > 20000 {
			return fmt.Errorf(`invalid iops "%d" in %+v: iops must be between "100" and "20000"`, r.Iops, r)
		}
	} else {
		if r.Iops != 0 {
			return fmt.Errorf(`invalid iops "%d" for volume type "%s" in %+v: iops must be "0" when type is "standard" or "gp2"`, r.Iops, r.Type, r)
		}

		if r.Type != "standard" && r.Type != "gp2" {
			return fmt.Errorf(`invalid type "%s" in %+v: type must be one of "standard", "gp2", "io1"`, r.Type, r)
		}
	}

	if r.Size <= 0 {
		return fmt.Errorf(`invalid size "%d" in %+v: size must be greater than "0"`, r.Size, r)
	}

	if r.Path == "" {
		return fmt.Errorf(`invalid path "%s" in %v: path cannot be empty`, r.Path, r)
	} else if regexp.MustCompile("^[a-zA-Z0-9/]*$").MatchString(r.Path) != true || strings.HasSuffix(r.Path, "/") || strings.HasPrefix(r.Path, "/") == false || strings.Contains(r.Path, "//") {
		return fmt.Errorf(`invalid path "%s" in %+v`, r.Path, r)
	}

	for _, device := range r.Devices {
		if strings.Compare(device, "/dev/xvdf") == -1 || strings.Compare(device, "/dev/xvdz") == 1 {
			return fmt.Errorf(`invalid device "%s" in %+v: device must be a value from "/dev/xvdf" to "/dev/xvdz"`, device, r)
		}
	}

	return nil
}

func ValidateRaid0Mounts(volumes []NodeVolumeMount, raid0s []Raid0Mount) error {
	paths := make(map[string]bool)
	devices := make(map[string]bool)

	// Populate any existing NodeVolumeMount items (assumes they're already Validated themselves).
	for _, volume := range volumes {
		paths[volume.Path] = true
		devices[volume.Device] = true
	}

	for _, raid0 := range raid0s {
		if err := raid0.Validate(); err != nil {
			return err
		}
		if paths[raid0.Path] == true {
			return fmt.Errorf("duplicate volumeMount or raid0Mount path detected (%s) - values must be unique", raid0.Path)
		}
		paths[raid0.Path] = true
		for _, device := range raid0.Devices {
			if devices[device] == true {
				return fmt.Errorf("duplicate volumeMount or raid0Mount device detected (%s) - values must be unique", device)
			}
			devices[device] = true
		}
	}

	return nil
}
