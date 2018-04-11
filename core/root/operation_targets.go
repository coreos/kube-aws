package root

// TODO: Add etcd
const (
	OperationTargetWorker       = "worker"
	OperationTargetControlPlane = "control-plane"
)

var OperationTargetNames = []string{
	OperationTargetControlPlane,
	OperationTargetWorker,
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

func (ts OperationTargets) IncludeControlPlane() bool {
	for _, t := range ts {
		if t == OperationTargetControlPlane {
			return true
		}
	}
	return false
}

func (ts OperationTargets) IncludeAll() bool {
	w := false
	c := false
	for _, t := range ts {
		w = w || t == OperationTargetWorker
		c = c || t == OperationTargetControlPlane
	}
	return w && c
}
