package api

import (
	"fmt"
)

type NATGatewayConfig struct {
	Identifier      `yaml:",inline"`
	EIPAllocationID string `yaml:"eipAllocationId,omitempty"`
}

func (c NATGatewayConfig) Validate() error {
	if c.HasIdentifier() && c.EIPAllocationID != "" {
		return fmt.Errorf("eipAllocationId can't be specified for a existing nat gatway. It is an user's responsibility to configure the nat gateway if one tried to reuse an existing one: %+v", c)
	}
	if err := c.Identifier.Validate(); err != nil {
		return fmt.Errorf("failed to validate id for nat gateway: %v", err)
	}
	return nil
}

// kube-aws manages at most one NAT gateway per subnet
type NATGateway interface {
	EIPAllocationIDRef() (string, error)
	EIPLogicalName() (string, error)
	IsConnectedToPrivateSubnet(Subnet) bool
	LogicalName() string
	ManageEIP() bool
	ManageNATGateway() bool
	ManageRoute() bool
	Ref() string
	PublicSubnetRef() (string, error)
	PrivateSubnets() []Subnet
	Validate() error
}

type natGatewayImpl struct {
	NATGatewayConfig
	privateSubnets []Subnet
	publicSubnet   Subnet
}

func NewManagedNATGateway(c NATGatewayConfig, private Subnet, public Subnet) NATGateway {
	return natGatewayImpl{
		NATGatewayConfig: c,
		privateSubnets:   []Subnet{private},
		publicSubnet:     public,
	}
}

func NewUnmanagedNATGateway(c NATGatewayConfig, private Subnet) NATGateway {
	return natGatewayImpl{
		NATGatewayConfig: c,
		privateSubnets:   []Subnet{private},
	}
}

func (g natGatewayImpl) LogicalName() string {
	name := ""
	for _, s := range g.privateSubnets {
		name = name + s.LogicalName()
	}
	return fmt.Sprintf("NatGateway%s", name)
}

func (g natGatewayImpl) ManageNATGateway() bool {
	allTrue := true
	allFalse := true
	for _, s := range g.privateSubnets {
		allTrue = allTrue && s.ManageNATGateway()
		allFalse = allFalse && !s.ManageNATGateway()
	}
	if allTrue {
		return true
	} else if allFalse {
		return false
	}

	panic(fmt.Sprintf("[bug] assertion failed: private subnets associated to this nat gateway(%+v) conflicts in their settings. kube-aws is confused and can't decide whether it should manage the nat gateway or not", g))
}

func (g natGatewayImpl) ManageEIP() bool {
	return g.ManageNATGateway() && g.EIPAllocationID == ""
}

func (g natGatewayImpl) ManageRoute() bool {
	allTrue := true
	allFalse := true
	for _, s := range g.privateSubnets {
		allTrue = allTrue && s.ManageRouteToNATGateway()
		allFalse = allFalse && !s.ManageRouteToNATGateway()
	}
	if allTrue {
		return true
	} else if allFalse {
		return false
	}

	panic(fmt.Sprintf("[bug] assertion failed: private subnets associated to this nat gateway(%+v) conflicts in their settings. kube-aws is confused and can't decide whether it should manage the route to nat gateway or not", g))
}

func (g natGatewayImpl) EIPLogicalName() (string, error) {
	if !g.ManageEIP() {
		return "", fmt.Errorf("[bug] assertion failed: EIPLogicalName shouldn't be called for NATGateway when an EIP is not going to be managed by kube-aws : %+v", g)
	}
	return fmt.Sprintf("%sEIP", g.LogicalName()), nil
}

func (g natGatewayImpl) EIPAllocationIDRef() (string, error) {
	if g.ManageEIP() {
		name, err := g.EIPLogicalName()
		if err != nil {
			return "", fmt.Errorf("failed to call EIPAlloationIDRef: %v", err)
		}
		return fmt.Sprintf(`{"Fn::GetAtt": ["%s", "AllocationId"]}`, name), nil
	}
	return fmt.Sprintf(`"%s"`, g.EIPAllocationID), nil
}

func (g natGatewayImpl) IsConnectedToPrivateSubnet(s Subnet) bool {
	for _, ps := range g.privateSubnets {
		if ps.LogicalName() == s.LogicalName() {
			return true
		}
	}
	return false
}

func (g natGatewayImpl) Ref() string {
	return g.Identifier.Ref(g.LogicalName)
}

func (g natGatewayImpl) PublicSubnetRef() (string, error) {
	if !g.ManageNATGateway() {
		return "", fmt.Errorf("[bug] assertion failed: PublicSubnetRef should't be called for an unmanaged NAT gateway: %+v", g)
	}
	return g.publicSubnet.Ref(), nil
}

func (g natGatewayImpl) PrivateSubnets() []Subnet {
	return g.privateSubnets
}

func (g natGatewayImpl) Validate() error {
	if err := g.NATGatewayConfig.Validate(); err != nil {
		return fmt.Errorf("failed to validate nat gateway: %v", err)
	}
	if !g.ManageNATGateway() {
		for i, s := range g.privateSubnets {
			if !s.HasIdentifier() && !s.RouteTable.HasIdentifier() && !s.NATGateway.HasIdentifier() {
				return fmt.Errorf("[bug] assertion failed: subnet #%d associated to preconfigured NGW be either a managed one with an unmanaged route table/ngw or an unmanaged one but it was not: subnet=%+v ngw=%+v", i, s, g)
			}
		}

		if !g.HasIdentifier() {
			return fmt.Errorf("Preconfigured NGW must have id or idFromStackOutput but it didn't: %+v", g)
		}

		if g.EIPAllocationID != "" {
			return fmt.Errorf("Preconfigured NGW must not have an eipAllocactionID but it didn't: %+v", g)
		}
	}
	return nil
}
