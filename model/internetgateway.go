package model

const internetGatewayLogicalName = "InternetGateway"

type InternetGateway struct {
	Identifier `yaml:",inline"`
}

func (c InternetGateway) LogicalName() string {
	return internetGatewayLogicalName
}

func (c InternetGateway) Ref() string {
	return c.Identifier.Ref(c.LogicalName())
}
