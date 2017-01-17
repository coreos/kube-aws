package model

import (
	"fmt"
	"strings"
)

const (
	publicSubnetLogicalName      = "Subnet"
	privateSubnetLogicalName     = "PrivateRouteTable"
	privateRouteTableLogicalName = "PrivateRouteTable"
	natGatewayLogicalName        = "NatGateway"
	eipAllocationLogicalName     = "EIPAllocation"
)

type Subnet struct {
	Identifier       `yaml:",inline"`
	AvailabilityZone string     `yaml:"availabilityZone,omitempty"`
	InstanceCIDR     string     `yaml:"instanceCIDR,omitempty"`
	NatGateway       NatGateway `yaml:"natGateway,omitempty"`
}

func (c Subnet) AvailabilityZoneLogicalName() string {
	return strings.Replace(strings.Title(c.AvailabilityZone), "-", "", -1)
}

func (c Subnet) NatGatewayLogicalName() string {
	return natGatewayLogicalName + c.AvailabilityZoneLogicalName()
}

func (c Subnet) NatGatewayRef() string {
	return c.NatGateway.Identifier.Ref(c.NatGatewayLogicalName())
}

func (c Subnet) NatGatewayEIPAllocationLogicalName() string {
	return c.NatGatewayLogicalName() + eipAllocationLogicalName
}

func (c Subnet) NatGatewayEIPAllocationRef() string {
	if c.NatGateway.EIPAllocation.IDFromStackOutput != "" || c.NatGateway.EIPAllocation.ID != "" {
		return c.NatGateway.EIPAllocation.Identifier.Ref("")
	} else {
		return fmt.Sprintf(`{ "Fn::GetAtt": [%q, "AllocationId"] }`, c.NatGatewayEIPAllocationLogicalName())
	}
}

type PublicSubnet struct {
	Subnet            `yaml:",inline"`
	MapPublicIp       bool       `yaml:"mapPublicIP,omitempty"`
	PrivateRouteTable RouteTable `yaml:"privateRouteTable,omitempty"`
}

func (c PublicSubnet) LogicalName() string {
	return publicSubnetLogicalName + c.AvailabilityZoneLogicalName()
}

func (c PublicSubnet) Ref() string {
	return c.Identifier.Ref(c.LogicalName())
}

func (c PublicSubnet) PrivateRouteTableLogicalName() string {
	return privateSubnetLogicalName + c.AvailabilityZoneLogicalName()
}

func (c PublicSubnet) PrivateRouteTableRef() string {
	return c.PrivateRouteTable.Identifier.Ref(c.PrivateRouteTableLogicalName())
}

type PrivateSubnet struct {
	Subnet     `yaml:",inline"`
	RouteTable RouteTable `yaml:"routeTable,omitempty"`
}

func (c PrivateSubnet) LogicalName(prefix string) string {
	return prefix + privateSubnetLogicalName + c.AvailabilityZoneLogicalName()
}

func (c PrivateSubnet) Ref(prefix string) string {
	return c.Identifier.Ref(c.LogicalName(prefix))
}

func (c PrivateSubnet) RouteTableLogicalName() string {
	return privateRouteTableLogicalName + c.AvailabilityZoneLogicalName()
}

func (c PrivateSubnet) RouteTableRef() string {
	return c.RouteTable.Identifier.Ref(c.RouteTableLogicalName())
}

// TODO: Might be better to make a map of subnets for use w/ "Fn::FindInMap" by AZ
func (c PrivateSubnet) PrivateRouteTableRef(publicSubnets []*PublicSubnet) string {
	for _, subnet := range publicSubnets {
		if subnet.AvailabilityZone == c.AvailabilityZone {
			return subnet.PrivateRouteTableRef()
		}
	}
	return ""
}

type NatGateway struct {
	Identifier    `yaml:",inline"`
	EIPAllocation EIPAllocation `yaml:"eip,omitempty"`
}

type EIPAllocation struct {
	Identifier `yaml:",inline"`
}
