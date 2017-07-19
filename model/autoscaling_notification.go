package model

import (
	"errors"
)

type AutoscalingNotification struct {
	IAMConfig                     `yaml:"iam,omitempty"`
	AutoscalingNotificationTarget `yaml:"target,omitempty"`
}

func (n AutoscalingNotification) RoleLogicalName() (string, error) {
	if !n.RoleManaged() {
		return "", errors.New("[BUG] RoleLogicalName should not be called when an existing autoscaling notification role was specified")
	}
	return "ASGNotificationRole", nil
}

func (n AutoscalingNotification) RoleArn() (string, error) {
	return n.IAMConfig.Role.ARN.OrGetAttArn(func() (string, error) { return n.RoleLogicalName() })
}

func (n AutoscalingNotification) RoleManaged() bool {
	return !n.IAMConfig.Role.HasArn()
}

func (n AutoscalingNotification) TargetLogicalName() (string, error) {
	if !n.TargetManaged() {
		return "", errors.New("[BUG] TargetLogicalName should not be called when an existing autoscaling notification target was specified")
	}
	return "ASGNotificationTarget", nil
}

func (n AutoscalingNotification) TargetArn() (string, error) {
	return n.AutoscalingNotificationTarget.ARN.OrGetAttArn(func() (string, error) { return n.TargetLogicalName() })
}

func (n AutoscalingNotification) TargetManaged() bool {
	return !n.AutoscalingNotificationTarget.HasArn()
}
