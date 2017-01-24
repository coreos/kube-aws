package model

import (
	"fmt"
	"strings"
)

type Subnet struct {
	//ID                string `yaml:"id,omitempty"`
	AvailabilityZone string     `yaml:"availabilityZone,omitempty"`
	InstanceCIDR     string     `yaml:"instanceCIDR,omitempty"`
	RouteTableID     string     `yaml:"routeTableId,omitempty"`
	ID               string     `yaml:"id,omitempty"`
	CustomName       string     `yaml:"name"`
	NatGateway       NatGateway `yaml:"natGateway,omitempty"`
	TopLevel         bool
}

func (c Subnet) AvailabilityZoneLogicalName() string {
	return strings.Replace(strings.Title(c.AvailabilityZone), "-", "", -1)
}

func (c Subnet) LogicalName() string {
	if c.TopLevel == true {
		if c.CustomName != "" {
			return c.CustomName
		}
		return "Subnet" + c.AvailabilityZoneLogicalName()
	}
	return "PrivateSubnet" + c.AvailabilityZoneLogicalName()
}

// Ref returns ID or ref to newly created resource
func (s Subnet) Ref() string {
	if s.ID != "" {
		return fmt.Sprintf("%q", s.ID)
	}
	return fmt.Sprintf(`{"Ref" : "%s"}`, s.LogicalName())
}

type NatGateway struct {
	ID              string `yaml:"id,omitempty"`
	EIPAllocationID string `yaml:"eipAllocationId,omitempty"`
}
