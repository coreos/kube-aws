package derived

import (
	"fmt"
	"github.com/coreos/kube-aws/model"
)

type Network interface {
	Subnets() []model.Subnet
	NATGateways() []model.NATGateway
	NATGatewayForSubnet(model.Subnet) (*model.NATGateway, error)
}

type networkImpl struct {
	subnets     []model.Subnet
	natGateways []model.NATGateway
}

func NewNetwork(subnets []model.Subnet, natGateways []model.NATGateway) Network {
	return networkImpl{
		subnets:     subnets,
		natGateways: natGateways,
	}
}

func (n networkImpl) Subnets() []model.Subnet {
	return n.subnets
}

func (n networkImpl) NATGateways() []model.NATGateway {
	return n.natGateways
}

func (n networkImpl) NATGatewayForSubnet(s model.Subnet) (*model.NATGateway, error) {
	for _, ngw := range n.NATGateways() {
		if ngw.IsConnectedToPrivateSubnet(s) {
			return &ngw, nil
		}
	}
	return nil, fmt.Errorf(`subnet "%s" doesn't have a corresponding nat gateway in: %v`, s.LogicalName(), n.natGateways)
}
