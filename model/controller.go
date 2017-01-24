package model

type Controller struct {
	AutoScalingGroup `yaml:"autoScalingGroup,omitempty"`
	Subnets          []*Subnet `yaml:"subnets,omitempty"`
}

func (c Controller) TopologyPrivate() bool {
	if len(c.Subnets) > 0 {
		return !c.Subnets[0].TopLevel
	}
	return false
}

func (c Controller) LogicalName() string {
	return "Controllers"
}

func (c Controller) SubnetLogicalNamePrefix() string {
	if c.TopologyPrivate() == true {
		return "Controller"
	}
	return ""
}

type ControllerElb struct {
	Private bool
	Subnets []*Subnet
}

func (c ControllerElb) SubnetLogicalNamePrefix() string {
	if c.Private == true {
		return "Controller"
	}
	return ""
}
