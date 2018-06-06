package model

import (
	"testing"
)

func TestVolumeMountSystemdMountName(t *testing.T) {

	c1 := VolumeMount{Path: "/ebs"}
	if c1.SystemdMountName() != "ebs" {
		t.Errorf("systemdMountName has produced an unexpected value '%+v' (expected '%+v') when using a 'path' value of '%+v'", c1.SystemdMountName(), "ebs", c1.Path)
	}

	c2 := VolumeMount{Path: "/ebs/sbe"}
	if c2.SystemdMountName() != "ebs-sbe" {
		t.Errorf("systemdMountName has produced an unexpected value '%+v' (expected '%+v') when using a 'path' value of '%+v'", c2.SystemdMountName(), "ebs-sbe", c2.Path)
	}

	c3 := VolumeMount{Path: "/az/AZ/09"}
	if c3.SystemdMountName() != "az-AZ-09" {
		t.Errorf("systemdMountName has produced an unexpected value '%+v' (expected '%+v') when using a 'path' value of '%+v'", c3.SystemdMountName(), "az-AZ-09", c3.Path)
	}
}

func TestVolumeMountValidate(t *testing.T) {

	c1 := VolumeMount{"gp2", 0, 100, "/dev/xvdf", "xfs", "/ebs", false}
	if c1.Validate() != nil {
		t.Errorf("validate should not return an error (%+v) with a valid configuration %+v", c1.Validate(), c1)
	}

	c2 := VolumeMount{"standard", 0, 100, "/dev/xvdf", "xfs", "/ebs", false}
	if c2.Validate() != nil {
		t.Errorf("validate should not return an error (%+v) with a valid configuration %+v", c2.Validate(), c2)
	}

	c3 := VolumeMount{"io1", 200, 100, "/dev/xvdf", "xfs", "/ebs", false}
	if c3.Validate() != nil {
		t.Errorf("validate should not return an error (%+v) with a valid configuration %+v", c3.Validate(), c3)
	}

	c4 := VolumeMount{"", 0, 100, "/dev/xvdf", "xfs", "/ebs", false}
	if c4.Validate() == nil {
		t.Errorf("validate should return a 'type' error for using an invalid 'type' value (%+v)", c4.Type)
	}

	c5 := VolumeMount{"gp2", 0, -5, "/dev/xvdf", "xfs", "/ebs", false}
	if c5.Validate() == nil {
		t.Errorf("validate should return a 'size' error for using an invalid 'size' value (%d)", c5.Size)
	}

	c6 := VolumeMount{"io1", 0, 100, "/dev/xvdf", "xfs", "/ebs", false}
	if c6.Validate() == nil {
		t.Errorf("validate should return a 'iops' error for using an invalid 'iops' value (%d)", c6.Iops)
	}

	c7 := VolumeMount{"io1", 1E9, 100, "/dev/xvdf", "xfs", "/ebs", false}
	if c7.Validate() == nil {
		t.Errorf("validate should return a 'size' error for using an invalid 'size' value (%d)", c7.Iops)
	}

	c8 := VolumeMount{"gp2", 0, 100, "/dev/xvda", "xfs", "/ebs", false}
	if c8.Validate() == nil {
		t.Errorf("validate should return a 'device' error for using an invalid 'device' value (%+v)", c8.Device)
	}

	c9 := VolumeMount{"gp2", 0, 100, "/dev/xvdF", "xfs", "/ebs", false}
	if c9.Validate() == nil {
		t.Errorf("validate should return a 'device' error for using an invalid 'device' value (%+v)", c9.Device)
	}

	c10 := VolumeMount{"gp2", 0, 100, "/dev/xvdf", "xfs", "/", false}
	if c10.Validate() == nil {
		t.Errorf("validate should return a 'path' error for using an invalid 'path' value (%+v)", c10.Path)
	}

	c11 := VolumeMount{"gp2", 0, 100, "/dev/xvdf", "xfs", "ebs", false}
	if c11.Validate() == nil {
		t.Errorf("validate should return a 'path' error for using an invalid 'path' value (%+v)", c11.Path)
	}

	c12 := VolumeMount{"gp2", 0, 100, "/dev/xvdf", "xfs", "/ebs/", false}
	if c12.Validate() == nil {
		t.Errorf("validate should return a 'path' error for using an invalid 'path' value (%+v)", c12.Path)
	}

	c13 := VolumeMount{"gp2", 0, 100, "/dev/xvdf", "xfs", "/ebs//sbe", false}
	if c13.Validate() == nil {
		t.Errorf("validate should return a 'path' error for using an invalid 'path' value (%+v)", c13.Path)
	}

	c14 := VolumeMount{"gp2", 0, 100, "/dev/xvdf", "xfs", "", false}
	if c14.Validate() == nil {
		t.Errorf("validate should return a 'path' error for using an invalid 'path' value (%+v)", c14.Path)
	}

	c15 := VolumeMount{"gp2", 0, 100, "/dev/xvdf", "xfs", "/ebs/sbe", false}
	if c15.Validate() != nil {
		t.Errorf("validate should not return an error (%+v) with a valid configuration %+v", c15.Validate(), c15)
	}
}

func TestVolumeMountValidateVolumeMounts(t *testing.T) {

	c1 := VolumeMount{"gp2", 0, 100, "/dev/xvdf", "xfs", "/ebs", false}
	if c1.Validate() != nil {
		t.Errorf("validate should not return an error (%+v) with a valid configuration %+v", c1.Validate(), c1)
	}

	c2 := VolumeMount{"gp2", 0, 100, "/dev/xvdf", "xfs", "/ebs2", false}
	if c2.Validate() != nil {
		t.Errorf("validate should not return an error (%+v) with a valid configuration %+v", c2.Validate(), c2)
	}

	c3 := VolumeMount{"gp2", 0, 100, "/dev/xvdg", "xfs", "/ebs", false}
	if c3.Validate() != nil {
		t.Errorf("validate should not return an error (%+v) with a valid configuration %+v", c3.Validate(), c3)
	}

	c4 := []VolumeMount{c2, c3}
	if ValidateVolumeMounts(c4) != nil {
		t.Errorf("validateEBSVolume should not return an error (%+v) with a valid configuration %+v", ValidateVolumeMounts(c4), c4)
	}

	c5 := []VolumeMount{c1, c2}
	if ValidateVolumeMounts(c5) == nil {
		t.Errorf("validate should return a 'device' duplication error for using duplicate 'device' values (%+v) (%+v)", c1.Device, c2.Device)
	}

	c6 := []VolumeMount{c1, c2}
	if ValidateVolumeMounts(c6) == nil {
		t.Errorf("validate should return a 'path' duplication error for using duplicate 'path' values (%+v) (%+v)", c1.Path, c2.Path)
	}
}
