package model

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
)

// NewEtcdNodes derives etcd nodes from user-provided etcd node configs
func NewEtcdNodes(nodeConfigs []api.EtcdNode, cluster EtcdCluster) ([]EtcdNode, error) {
	count := cluster.NodeCount()

	result := make([]EtcdNode, count)
	for etcdIndex := 0; etcdIndex < count; etcdIndex++ {

		//Round-robin etcd instances across all available subnets
		subnetIndex := etcdIndex % len(cluster.Subnets())
		subnet := cluster.Subnets()[subnetIndex]

		nodeConfig := api.EtcdNode{}
		if len(nodeConfigs) == count {
			nodeConfig = nodeConfigs[etcdIndex]
		}

		if subnet.ManageNATGateway() {
			ngw, err := cluster.NATGatewayForSubnet(subnet)

			if err != nil {
				return nil, fmt.Errorf("failed to determine nat gateway for subnet %s: %v", subnet.LogicalName(), err)
			}

			result[etcdIndex] = NewEtcdNodeDependsOnManagedNGW(cluster, etcdIndex, nodeConfig, subnet, *ngw)
		} else {
			result[etcdIndex] = NewEtcdNode(cluster, etcdIndex, nodeConfig, subnet)
		}

		//http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-instance-addressing.html#concepts-private-addresses

	}
	return result, nil
}
