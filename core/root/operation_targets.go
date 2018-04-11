package root

import "strings"

// TODO: Add etcd
const (
	OperationTargetWorker       = "worker"
	OperationTargetControlPlane = "control-plane"
	OperationTargetEtcd         = "etcd"
	OperationTargetNetwork      = "network"
)

var OperationTargetNames = []string{
	OperationTargetControlPlane,
	OperationTargetEtcd,
	OperationTargetWorker,
	OperationTargetNetwork,
}

type OperationTargets []string

func AllOperationTargetsAsStringSlice() []string {
	return OperationTargetNames
}

func AllOperationTargets() OperationTargets {
	return OperationTargets(OperationTargetNames)
}

func OperationTargetsFromStringSlice(targets []string) OperationTargets {
	return OperationTargets(targets)
}

func (ts OperationTargets) IncludeWorker() bool {
	for _, t := range ts {
		if t == OperationTargetWorker {
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

func (ts OperationTargets) IncludeAll() bool {
	w := false
	c := false
	e := false
	n := false
	for _, t := range ts {
		w = w || t == OperationTargetWorker
		c = c || t == OperationTargetControlPlane
		e = e || t == OperationTargetEtcd
		n = n || t == OperationTargetNetwork
	}
	return w && c && e && n
}

func (ts OperationTargets) String() string {
	return strings.Join(ts, ", ")
}
