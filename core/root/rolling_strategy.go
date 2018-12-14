package root

import (
	"fmt"
	"strings"

	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
)

// returns NodePoolRolling strategy string to be used in stack-template
func (p nodePool) NodePoolRollingStrategy() string {
	return p.nodePool.NodePoolConfig.WorkerNodePool.NodePoolRollingStrategy
}

// NodePoolAvailabilityZoneDependencies produces a list of NodePool logical names that the present nodepool depends upon
// when using the 'AvailabilityZone' NodePoolRollingStrategy.
// Only other pools using the 'AvailabilityZone' strategy are considered.
// Only nodepools containing a single AZ can use this strategy!
// Returns a comma separated quoted list, e.g. "pool1","pool2","pool3"
func (c Cluster) nodePoolAvailabilityZoneDependencies(pool nodePool, subnets api.Subnets) (string, error) {
	poolConfig := pool.nodePool.NodePoolConfig

	var order []string
	order, err := c.azOrder()
	if err != nil {
		return "", fmt.Errorf("can't resolve nodepool availability zone ordering dependencies for %s: %v", poolConfig.NodePoolName, err)
	}
	logger.Debugf("AZ Rollout order: %v", order)

	// only the first subnet is used to determine the az asscociated with the nodepool.
	position, err := azPosition(order, poolConfig.Subnets[0].AvailabilityZone)
	if err != nil {
		logger.Debugf("There was the following error looking up nodepool azPosition: %v", err)
		return "", err
	}
	if position == 0 {
		// a nodePool with position 0 doesn't have any other nodepool dependencies
		logger.Debugf("The AZ for nodepool %s is first in the list, so it does not have any dependencies", poolConfig.NodePoolName)
		return "", nil
	}

	return `"` + strings.Join(c.allNodePoolLogicalNamesinAZ(order[position-1]), `","`) + `"`, nil
}

// rolloutAZOrder works out an order for availability zones from the order of the nodepool stacks were loaded from the cluster.yaml.
// It also provides some validation - preventing users from selecting subnets in separate az's which would be impossible to order by placing dependencies
// at the nodepool stack level.
func (c Cluster) azOrder() ([]string, error) {
	var azOrder []string
	seen := make(map[string]bool)
	for _, pool := range c.nodePoolStacks {
		poolConfig := pool.NodePoolConfig
		if poolConfig.NodePoolRollingStrategy == "AvailabilityZone" {
			if len(poolConfig.Subnets) == 0 {
				return azOrder, fmt.Errorf("worker nodepool %s has 'AvailabilityZone' rolling strategy but has no subnets", poolConfig.NodePoolName)
			}
			for _, subnet := range poolConfig.Subnets {
				if subnet.AvailabilityZone == "" {
					return azOrder, fmt.Errorf("worker nodepool %s can not use the 'AvailabilityZone' rolling strategy because its subnet %s has an empty availability zone", poolConfig.NodePoolName, subnet.Name)
				}
				if subnet.AvailabilityZone != poolConfig.Subnets[0].AvailabilityZone {
					return azOrder, fmt.Errorf("worker nodepool %s can't have subnets in different availability zones and also use the 'AvailabilityZone' rolling strategy", poolConfig.NodePoolName)
				}
				if !seen[subnet.AvailabilityZone] {
					logger.Debugf("added new az %s to azdeployment order", subnet.AvailabilityZone)
					azOrder = append(azOrder, subnet.AvailabilityZone)
					seen[subnet.AvailabilityZone] = true
				}
			}
		}
	}
	return azOrder, nil
}

// azPosition tells us which integer position a particular az lies within an list
func azPosition(azList []string, az string) (int, error) {
	for index, value := range azList {
		if value == az {
			logger.Debugf("az %s is position %d in the azorder", az, index)
			return index, nil
		}
	}
	return 0, fmt.Errorf("could not find az %s in the azorder list: %v", az, azList)
}

// allNodePoolsinAZ returns a slice of nodepool names with subnets within a given availability zone
func (c Cluster) allNodePoolLogicalNamesinAZ(az string) []string {
	var poolNames []string
	for _, pool := range c.nodePoolStacks {
		poolConfig := pool.NodePoolConfig
		logger.Debugf("found the following AZ in nodepool %s: '%s'", poolConfig.NodePoolName, poolConfig.Subnets[0].AvailabilityZone)
		if poolConfig.NodePoolRollingStrategy == "AvailabilityZone" && poolConfig.Subnets[0].AvailabilityZone == az {
			poolNames = append(poolNames, poolConfig.NodePoolLogicalName())
		}
	}
	logger.Debugf("The following nodepools are in AZ %s: %v", az, poolNames)
	return poolNames
}
