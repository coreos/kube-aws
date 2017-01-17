package model

type RouteTable struct {
	Identifier `yaml:",inline"`
}

func (c RouteTable) LogicalName() string {
	return "PublicRouteTable"
}

func (c RouteTable) Ref() string {
	return c.Identifier.Ref(c.LogicalName())
}
