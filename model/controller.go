package model

type Controller struct {
	AutoScalingGroup `yaml:"autoScalingGroup,omitempty"`
	PrivateSubnets   []*PrivateSubnet `yaml:"privateSubnets,omitempty"`
}

func (c Controller) TopologyPrivate() bool {
	return len(c.PrivateSubnets) > 0
}
