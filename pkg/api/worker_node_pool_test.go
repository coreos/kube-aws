package api

import (
	"testing"
)

func intp(v int) *int {
	return &v
}

func nodePoolWithCount(n int) WorkerNodePool {
	return WorkerNodePool{
		EC2Instance: EC2Instance{
			Count: n,
		},
	}
}

func nodePoolWithMinSize(n int) WorkerNodePool {
	return WorkerNodePool{
		AutoScalingGroup: AutoScalingGroup{
			MinSize: intp(n),
		},
	}
}

func nodePoolWithMaxSize(n int) WorkerNodePool {
	return WorkerNodePool{
		AutoScalingGroup: AutoScalingGroup{
			MaxSize: n,
		},
	}
}

func nodePoolWithCountAndMinInService(n int, minInService int) WorkerNodePool {
	return WorkerNodePool{
		EC2Instance: EC2Instance{
			Count: n,
		},
		AutoScalingGroup: AutoScalingGroup{
			RollingUpdateMinInstancesInService: intp(minInService),
		},
	}
}

func TestNodePoolMinCount(t *testing.T) {
	c1 := nodePoolWithCount(0)
	if c1.MinCount() != 0 {
		t.Errorf("min count should be 0 but was %d in %+v", c1.MinCount(), c1)
	}

	c2 := nodePoolWithCount(2)
	if c2.MinCount() != 2 {
		t.Errorf("min count should be 2 but was %d in %+v", c2.MinCount(), c2)
	}

	c3 := nodePoolWithMinSize(0)
	if c3.MinCount() != 0 {
		t.Errorf("min count should be 0 but was %d in %+v", c3.MinCount(), c3)
	}

	c4 := nodePoolWithMinSize(2)
	if c4.MinCount() != 2 {
		t.Errorf("min count should be 2 but was %d in %+v", c4.MinCount(), c4)
	}
}

func TestNodePoolMaxCount(t *testing.T) {
	c1 := nodePoolWithCount(0)
	if c1.MaxCount() != 0 {
		t.Errorf("max count should be 0 but was %d in %+v", c1.MaxCount(), c1)
	}

	c2 := nodePoolWithCount(2)
	if c2.MaxCount() != 2 {
		t.Errorf("max count should be 2 but was %d in %+v", c2.MaxCount(), c2)
	}

	c3 := nodePoolWithMaxSize(0)
	if c3.MaxCount() != 0 {
		t.Errorf("max count should be 0 but was %d in %+v", c3.MaxCount(), c3)
	}

	c4 := nodePoolWithMaxSize(2)
	if c4.MaxCount() != 2 {
		t.Errorf("max count should be 2 but was %d in %+v", c4.MaxCount(), c4)
	}
}

func TestNodePoolRollingUpdateMinInstancesInService(t *testing.T) {
	c1 := nodePoolWithCount(0)
	if c1.RollingUpdateMinInstancesInService() != 0 {
		t.Errorf("min instances in service should be 0 but was %d in %+v", c1.RollingUpdateMinInstancesInService(), c1)
	}

	c2 := nodePoolWithCount(2)
	if c2.RollingUpdateMinInstancesInService() != 1 {
		t.Errorf("min instances in service should be 2 but was %d in %+v", c2.RollingUpdateMinInstancesInService(), c2)
	}

	c3 := nodePoolWithMinSize(2)
	if c3.RollingUpdateMinInstancesInService() != 1 {
		t.Errorf("min instances in service should be 2 but was %d in %+v", c3.RollingUpdateMinInstancesInService(), c3)
	}

	c4 := nodePoolWithMaxSize(2)
	if c4.RollingUpdateMinInstancesInService() != 1 {
		t.Errorf("min instances in service should be 2 but was %d in %+v", c4.RollingUpdateMinInstancesInService(), c4)
	}

	c5 := nodePoolWithCountAndMinInService(2, 0)
	if c5.RollingUpdateMinInstancesInService() != 0 {
		t.Errorf("min instances in service should be 0 but was %d in %+v", c5.RollingUpdateMinInstancesInService(), c5)
	}

	c6 := nodePoolWithCountAndMinInService(0, 2)
	if c6.RollingUpdateMinInstancesInService() != 2 {
		t.Errorf("min instances in service should be 2 but was %d in %+v", c6.RollingUpdateMinInstancesInService(), c6)
	}
}
