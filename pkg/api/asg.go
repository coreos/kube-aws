package api

import (
	"fmt"
)

// Configuration specific to auto scaling groups
type AutoScalingGroup struct {
	MinSize                            *int `yaml:"minSize,omitempty"`
	MaxSize                            int  `yaml:"maxSize,omitempty"`
	RollingUpdateMinInstancesInService *int `yaml:"rollingUpdateMinInstancesInService,omitempty"`
	UnknownKeys                        `yaml:",inline"`
}

func (asg AutoScalingGroup) Validate() error {
	if asg.MinSize != nil && *asg.MinSize < 0 {
		return fmt.Errorf("`autoScalingGroup.minSize` must be zero or greater if specified")
	}
	if asg.MaxSize < 0 {
		return fmt.Errorf("`autoScalingGroup.maxSize` must be zero or greater if specified")
	}
	if asg.MinSize != nil && *asg.MinSize > asg.MaxSize {
		return fmt.Errorf("`autoScalingGroup.minSize` (%d) must be less than or equal to `autoScalingGroup.maxSize` (%d), if you have specified only minSize please specify maxSize as well",
			asg.MinSize, asg.MaxSize)
	}
	if asg.RollingUpdateMinInstancesInService != nil && *asg.RollingUpdateMinInstancesInService < 0 {
		return fmt.Errorf("`autoScalingGroup.rollingUpdateMinInstancesInService` must be greater than or equal to 0 but was %d", *asg.RollingUpdateMinInstancesInService)
	}
	return nil
}
