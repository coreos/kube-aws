package api

import (
	"fmt"
	"strings"
)

type Subnet struct {
	Identifier       `yaml:",inline"`
	AvailabilityZone string           `yaml:"availabilityZone,omitempty"`
	Name             string           `yaml:"name,omitempty"`
	InstanceCIDR     string           `yaml:"instanceCIDR,omitempty"`
	InternetGateway  InternetGateway  `yaml:"internetGateway,omitempty"`
	NATGateway       NATGatewayConfig `yaml:"natGateway,omitempty"`
	Private          bool             `yaml:"private,omitempty"`
	RouteTable       RouteTable       `yaml:"routeTable,omitempty"`
}

func NewPublicSubnet(az string, cidr string) Subnet {
	return Subnet{
		AvailabilityZone: az,
		InstanceCIDR:     cidr,
		Private:          false,
	}
}

func NewPrivateSubnet(az string, cidr string) Subnet {
	return Subnet{
		AvailabilityZone: az,
		InstanceCIDR:     cidr,
		Private:          true,
	}
}

func NewExistingPrivateSubnet(az string, id string) Subnet {
	return Subnet{
		Identifier: Identifier{
			ID: id,
		},
		AvailabilityZone: az,
		Private:          true,
	}
}

func NewPublicSubnetWithPreconfiguredRouteTable(az string, cidr string, rtb string) Subnet {
	return Subnet{
		AvailabilityZone: az,
		InstanceCIDR:     cidr,
		Private:          false,
		RouteTable: RouteTable{
			Identifier: Identifier{
				ID: rtb,
			},
		},
		InternetGateway: InternetGateway{},
	}
}

func NewPrivateSubnetWithPreconfiguredRouteTable(az string, cidr string, rtb string) Subnet {
	return Subnet{
		AvailabilityZone: az,
		InstanceCIDR:     cidr,
		Private:          true,
		RouteTable: RouteTable{
			Identifier: Identifier{
				ID: rtb,
			},
		},
		NATGateway: NATGatewayConfig{},
	}
}

func NewPrivateSubnetWithPreconfiguredNATGateway(az string, cidr string, ngw string) Subnet {
	return Subnet{
		AvailabilityZone: az,
		InstanceCIDR:     cidr,
		Private:          true,
		RouteTable:       RouteTable{},
		NATGateway: NATGatewayConfig{
			Identifier: Identifier{
				ID: ngw,
			},
		},
	}
}

func NewPrivateSubnetWithPreconfiguredNATGatewayEIP(az string, cidr string, alloc string) Subnet {
	return Subnet{
		AvailabilityZone: az,
		InstanceCIDR:     cidr,
		Private:          true,
		RouteTable:       RouteTable{},
		NATGateway: NATGatewayConfig{
			EIPAllocationID: alloc,
		},
	}
}

func NewImportedPrivateSubnet(az string, name string) Subnet {
	return Subnet{
		Identifier: Identifier{
			IDFromStackOutput: name,
		},
		AvailabilityZone: az,
		Private:          true,
	}
}

func NewExistingPublicSubnet(az string, id string) Subnet {
	return Subnet{
		Identifier: Identifier{
			ID: id,
		},
		AvailabilityZone: az,
		Private:          false,
	}
}

func NewImportedPublicSubnet(az string, name string) Subnet {
	return Subnet{
		Identifier: Identifier{
			IDFromStackOutput: name,
		},
		AvailabilityZone: az,
		Private:          false,
	}
}

func NewPublicSubnetFromFn(az string, fn string) Subnet {
	return Subnet{
		Identifier: Identifier{
			IDFromFn: fn,
		},
		AvailabilityZone: az,
		Private:          false,
	}
}

func NewPrivateSubnetFromFn(az string, fn string) Subnet {
	return Subnet{
		Identifier: Identifier{
			IDFromFn: fn,
		},
		AvailabilityZone: az,
		Private:          true,
	}
}

func (s *Subnet) Public() bool {
	return !s.Private
}

func (s *Subnet) Validate() error {
	if err := s.Identifier.Validate(); err != nil {
		return fmt.Errorf("failed to validate id for subnet: %v", err)
	}
	if err := s.RouteTable.Validate(); err != nil {
		return fmt.Errorf("failed to validate route table for subnet: %v", err)
	}
	if err := s.NATGateway.Validate(); err != nil {
		return fmt.Errorf("failed to validate nat gateway for subnet: %v", err)
	}
	return nil
}

func (s *Subnet) MapPublicIPs() bool {
	return !s.Private
}

func (s *Subnet) LogicalName() string {
	result, err := s.LogicalNameOrErr()

	if err != nil {
		panic(err)
	}

	return result
}

func (s *Subnet) LogicalNameOrErr() (string, error) {
	if s.Name != "" {
		return strings.Replace(strings.Title(s.Name), "-", "", -1), nil
	}
	return "", fmt.Errorf("name must be set for a subnet: %+v", *s)
}

func (s *Subnet) RouteTableID() string {
	return s.RouteTable.ID
}

// ManageNATGateway returns true if a NAT gateway for this subnet must be created or updated by kube-aws
// kube-aws creates or updates a NAT gateway if:
// * the subnet is private and
// * the subnet is going to be managed by kube-aws(an existing subnet is NOT specified) and
// * the route table for the subnet is going to be managed by kube-aws(an existing subnet is NOT specified) and
// * an existing NAT gateway ID is not specified to be reused
func (s *Subnet) ManageNATGateway() bool {
	return s.managePrivateRouteTable() && !s.NATGateway.HasIdentifier()
}

// ManageRouteToNATGateway returns true if a route to a NAT gateway for this subnet must be created or updated by kube-aws
// kube-aws creates or updates a NAT gateway if:
// * the NGW is going to be managed or
// * an existing NAT gateway ID is specified
func (s *Subnet) ManageRouteToNATGateway() bool {
	return s.managePrivateRouteTable()
}

func (s *Subnet) managePrivateRouteTable() bool {
	return s.Private && s.ManageRouteTable()
}

// ManageRouteTable returns true if a route table for this subnet must be created or updated by kube-aws
// kube-aws creates a route table if and only if the subnet is also going to be managed and an existing route table for it isn't specified
func (s *Subnet) ManageRouteTable() bool {
	return s.ManageSubnet() && !s.RouteTable.HasIdentifier()
}

// ManageRouteToInternet returns true if a route from this subnet to an IGW must be created or updated by kube-aws
// kube-aws creates a route to an IGW for an subnet if and only if:
// * the subnet is public and
// * the subnet is going to be managed by kube-aws and
// * the route table is going to be managed by kube-aws
// In other words, kube-aws won't create or update a route to an IGW if:
// * the subnet is private or
// * an existing subnet is used or
// * an existing route table is used
func (s *Subnet) ManageRouteToInternet() bool {
	return s.Public() && s.ManageSubnet() && s.ManageRouteTable()
}

// ManageSubnet returns true if this subnet must be managed(created or updated) by kube-aws
// kube-aws creates a subnet if subnet.id and subnet.idFromStackOutput are not specified
func (s *Subnet) ManageSubnet() bool {
	return !s.HasIdentifier()
}

// Ref returns ID or ref to newly created resource
func (s *Subnet) Ref() string {
	return s.Identifier.Ref(s.LogicalName)
}

// RouteTableLogicalName represents the name of the route table to which this subnet is associated.
func (s *Subnet) RouteTableLogicalName() (string, error) {
	// There should be no need to call this func if the route table isn't going to be created/updated by kube-aws
	if !s.ManageRouteTable() {
		return "", fmt.Errorf("[bug] assertion failed: RouteTableLogicalName() must be called if and only if ManageRouteTable() returns true")
	}
	return s.subnetSpecificResourceLogicalName("RouteTable"), nil
}

func (s *Subnet) InternetGatewayRouteLogicalName() string {
	return s.subnetSpecificResourceLogicalName("RouteToInternet")
}

func (s *Subnet) NATGatewayRouteLogicalName() string {
	return s.subnetSpecificResourceLogicalName("RouteToNatGateway")
}

func (s *Subnet) subnetSpecificResourceLogicalName(resourceName string) string {
	return fmt.Sprintf("%s%s", s.LogicalName(), resourceName)
}

func (s *Subnet) RouteTableRef() (string, error) {
	return s.RouteTable.RefOrError(s.RouteTableLogicalName)
}

// kube-aws manages at most one route table per subnet
// If ID or IDFromStackOutput is non-zero, kube-aws doesn't manage the route table but its users' responsibility to
// provide properly configured one to be reused by kube-aws.
// More concretely:
// * If an user is going to reuse an existing route table for a private subnet, it must have a route to a NAT gateway
//   * A NAT gateway can be either a classical one with a NAT EC2 instance or an AWS-managed one
// * IF an user is going to reuse an existing route table for a public subnet, it must have a route to an Internet gateway
type RouteTable struct {
	Identifier `yaml:",inline"`
}
