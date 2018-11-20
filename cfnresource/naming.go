package cfnresource

import (
	"fmt"
)

func ValidateUnstableRoleNameLength(clusterName string, nestedStackLogicalName string, managedIAMRoleName string, region string, strict bool) error {
	if strict {
		name := managedIAMRoleName
		if len(name) > 64 {
			return fmt.Errorf("IAM role name(=%s) will be %d characters long. It exceeds the AWS limit of 64 characters", name, len(name))
		}
	} else {
		name := fmt.Sprintf("%s-%s-PRK1CVQNY7XZ-%s-%s", clusterName, nestedStackLogicalName, region, managedIAMRoleName)
		if len(name) > 64 {
			limit := 64 - len(name) + len(clusterName) + len(nestedStackLogicalName) + len(managedIAMRoleName)
			return fmt.Errorf("IAM role name(=%s) will be %d characters long. It exceeds the AWS limit of 64 characters: cluster name(=%s) + nested stack name(=%s) + managed iam role name(=%s) should be less than or equal to %d", name, len(name), clusterName, nestedStackLogicalName, managedIAMRoleName, limit)
		}
	}
	return nil
}

func ValidateStableRoleNameLength(clusterName string, managedIAMRoleName string, region string, strict bool) error {
	// include cluster name in the managed role
	// enables multiple clusters in the same account and region to have mirrored configuration without clashes
	if strict {
		name := managedIAMRoleName
		if len(name) > 64 {
			return fmt.Errorf("IAM role name(=%s) will be %d characters long. It exceeds the AWS limit of 64 characters", name, len(name))
		}
	} else {
		name := fmt.Sprintf("%s-%s-%s", clusterName, region, managedIAMRoleName)
		if len(name) > 64 {
			limit := 64 - len(name) + len(managedIAMRoleName)
			return fmt.Errorf("IAM role name(=%s) will be %d characters long. It exceeds the AWS limit of 64 characters: clusterName(=%s) + region name(=%s) + managed iam role name(=%s) should be less than or equal to %d", name, len(name), clusterName, region, managedIAMRoleName, limit)
		}
	}
	return nil
}
