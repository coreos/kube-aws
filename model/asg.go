package model

import (
	"fmt"
)

// Configuration specific to auto scaling groups
type AutoScalingGroup struct {
	MinSize                            int `yaml:"minSize,omitempty"`
	MaxSize                            int `yaml:"maxSize,omitempty"`
	RollingUpdateMinInstancesInService int `yaml:"rollingUpdateMinInstancesInService,omitempty"`
}

func (asg AutoScalingGroup) Valid() error {
	if asg.MinSize < 0 {
		return fmt.Errorf("`autoScalingGroup.minSize` must be zero or greater if specified")
	}
	if asg.MaxSize < 0 {
		return fmt.Errorf("`autoScalingGroup.maxSize` must be zero or greater if specified")
	}
	if asg.MinSize > asg.MaxSize {
		return fmt.Errorf("`autoScalingGroup.minSize` (%d) must be less than or equal to `autoScalingGroup.maxSize` (%d), if you have specified only minSize please specify maxSize as well",
			asg.MinSize, asg.MaxSize)
	}
	return nil
}
