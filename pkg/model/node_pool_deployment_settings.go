package model

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
)

type NodePoolDeploymentSettings struct {
	api.WorkerNodePool
	api.Experimental
	api.DeploymentSettings
}

func (c NodePoolDeploymentSettings) WorkerSecurityGroupRefs() []string {
	refs := []string{}

	if c.Experimental.LoadBalancer.Enabled {
		for _, sgId := range c.Experimental.LoadBalancer.SecurityGroupIds {
			refs = append(refs, fmt.Sprintf(`"%s"`, sgId))
		}
	}

	if c.Experimental.TargetGroup.Enabled {
		for _, sgId := range c.Experimental.TargetGroup.SecurityGroupIds {
			refs = append(refs, fmt.Sprintf(`"%s"`, sgId))
		}
	}

	for _, sgId := range c.SecurityGroupIds {
		refs = append(refs, fmt.Sprintf(`"%s"`, sgId))
	}

	return refs
}

func (c NodePoolDeploymentSettings) StackTags() map[string]string {
	tags := map[string]string{}

	for k, v := range c.DeploymentSettings.StackTags {
		tags[k] = v
	}

	return tags
}

func (c NodePoolDeploymentSettings) Validate() error {
	sgRefs := c.WorkerSecurityGroupRefs()
	numSGs := len(sgRefs)

	if numSGs > 4 {
		return fmt.Errorf("number of user provided security groups must be less than or equal to 4 but was %d (actual EC2 limit is 5 but one of them is reserved for kube-aws) : %v", numSGs, sgRefs)
	}

	return nil
}
