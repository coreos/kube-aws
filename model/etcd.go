package model

type Etcd struct {
	Subnets []*Subnet `yaml:"subnets,omitempty"`
}

func (c Etcd) TopologyPrivate() bool {
	return len(c.Subnets) > 0
}

type EtcdInstance struct {
	Subnet Subnet
}

func (c EtcdInstance) SubnetLogicalNamePrefix() string {
	if c.Subnet.TopLevel == false {
		return "Etcd"
	}
	return ""
}
