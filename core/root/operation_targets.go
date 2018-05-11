package root

import "strings"

// TODO: Add etcd
const (
	OperationTargetControlPlane = "control-plane"
	OperationTargetEtcd         = "etcd"
	OperationTargetNetwork      = "network"
	OperationTargetAll          = "all"
)

var OperationTargetNames = []string{
	OperationTargetControlPlane,
	OperationTargetEtcd,
	OperationTargetNetwork,
}

type OperationTargets []string

func AllOperationTargetsAsStringSlice() []string {
	return []string{"all"}
}

func AllOperationTargetsWith(nodePoolNames []string) OperationTargets {
	ts := []string{}
	ts = append(ts, OperationTargetNames...)
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

func (ts OperationTargets) IncludeNetwork() bool {
	for _, t := range ts {
		if t == OperationTargetNetwork {
			return true
		}
	}
	return false
}

func (ts OperationTargets) IncludeControlPlane() bool {
	for _, t := range ts {
		if t == OperationTargetControlPlane {
			return true
		}
	}
	return false
}

func (ts OperationTargets) IncludeEtcd() bool {
	for _, t := range ts {
		if t == OperationTargetEtcd {
			return true
		}
	}
	return false
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
