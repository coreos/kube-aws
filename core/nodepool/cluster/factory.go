package cluster

import (
	controlplane "github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	"github.com/kubernetes-incubator/kube-aws/core/nodepool/config"
)

func ClusterRefFromBytes(bytes []byte, main *controlplane.Config, awsDebug bool) (*ClusterRef, error) {
	provided, err := config.ClusterFromBytes(bytes, main)
	if err != nil {
		return nil, err
	}
	c := NewClusterRef(provided, awsDebug)
	return c, nil
}
