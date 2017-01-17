package model

const hostedZoneLogicalName = "HostedZone"

type HostedZone struct {
	Identifier `yaml:",inline"`
}

func (c HostedZone) LogicalName() string {
	return hostedZoneLogicalName
}

func (c HostedZone) Ref() string {
	return c.Identifier.Ref(c.LogicalName())
}
