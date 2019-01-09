package root

import "strings"

// TODO: Add etcd
const (
	OperationTargetAll = "all"
)

type OperationTargets []string

func AllOperationTargetsAsStringSlice() []string {
	return []string{"all"}
}

func AllOperationTargetsWith(nodePoolNames []string, operationTargetNames []string) OperationTargets {
	ts := []string{}
	ts = append(ts, operationTargetNames...)
	ts = append(ts, nodePoolNames...)
	return OperationTargets(ts)
}

func OperationTargetsFromStringSlice(targets []string) OperationTargets {
	return OperationTargets(targets)
}

func (ts OperationTargets) IncludeWorker(nodePoolName string) bool {
	for _, t := range ts {
		if t == nodePoolName {
			return true
		}
	}
	return false
}

func (ts OperationTargets) IncludeNetwork(networkStackName string) bool {
	for _, t := range ts {
		if t == networkStackName {
			return true
		}
	}
	return false
}

func (ts OperationTargets) IncludeControlPlane(controlPlaneStackName string) bool {
	for _, t := range ts {
		if t == controlPlaneStackName {
			return true
		}
	}
	return false
}

func (ts OperationTargets) IncludeEtcd(etcdStackName string) bool {
	for _, t := range ts {
		if t == etcdStackName {
			return true
		}
	}
	return false
}
func (ts OperationTargets) IncludeAll(c *clusterImpl) bool {
	return ts.IncludeNetwork(c.Network().NetworkStackName()) &&
		ts.IncludeControlPlane(c.ControlPlane().ControlPlaneStackName()) &&
		ts.IncludeEtcd(c.Etcd().EtcdStackName())
}

func (ts OperationTargets) IsAll() bool {
	for _, t := range ts {
		if t == OperationTargetAll {
			return true
		}
	}
	return false
}

func (ts OperationTargets) String() string {
	return strings.Join(ts, ", ")
}
