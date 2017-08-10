package cfnresource

import (
	"fmt"
)

func ValidateUnstableRoleNameLength(clusterName string, nestedStackLogicalName string, managedIAMRoleName string, region string) error {
	name := fmt.Sprintf("%s-%s-PRK1CVQNY7XZ-%s-%s", clusterName, nestedStackLogicalName, region, managedIAMRoleName)
	if len(name) > 64 {
		limit := 64 - len(name) + len(clusterName) + len(nestedStackLogicalName) + len(managedIAMRoleName)
		return fmt.Errorf("IAM role name(=%s) will be %d characters long. It exceeds the AWS limit of 64 characters: cluster name(=%s) + nested stack name(=%s) + managed iam role name(=%s) should be less than or equal to %d", name, len(name), clusterName, nestedStackLogicalName, managedIAMRoleName, limit)
	}
	return nil
}

func ValidateStableRoleNameLength(managedIAMRoleName string, region string) error {
	name := fmt.Sprintf("%s-%s", region, managedIAMRoleName)
	if len(name) > 64 {
		limit := 64 - len(name) + len(managedIAMRoleName)
		return fmt.Errorf("IAM role name(=%s) will be %d characters long. It exceeds the AWS limit of 64 characters: region name(=%s) + managed iam role name(=%s) should be less than or equal to %d", name, len(name), region, managedIAMRoleName, limit)
	}
	return nil
}
