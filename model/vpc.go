package model

const vpcLogicalName = "VPC"

type VPC struct {
	Identifier `yaml:",inline"`
	CIDR       string `yaml:"cidr,omitempty"`
}

func (c VPC) LogicalName() string {
	return vpcLogicalName
}

func (c VPC) Ref() string {
	return c.Identifier.Ref(c.LogicalName())
}
