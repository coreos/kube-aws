package model

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
)

type Network interface {
	Subnets() []api.Subnet
	NATGateways() []api.NATGateway
	NATGatewayForSubnet(api.Subnet) (*api.NATGateway, error)
}

type networkImpl struct {
	subnets     []api.Subnet
	natGateways []api.NATGateway
}

func NewNetwork(subnets []api.Subnet, natGateways []api.NATGateway) Network {
	return networkImpl{
		subnets:     subnets,
		natGateways: natGateways,
	}
}

func (n networkImpl) Subnets() []api.Subnet {
	return n.subnets
}

func (n networkImpl) NATGateways() []api.NATGateway {
	return n.natGateways
}

func (n networkImpl) NATGatewayForSubnet(s api.Subnet) (*api.NATGateway, error) {
	for _, ngw := range n.NATGateways() {
		if ngw.IsConnectedToPrivateSubnet(s) {
			return &ngw, nil
		}
	}
	return nil, fmt.Errorf(`subnet "%s" doesn't have a corresponding nat gateway in: %v`, s.LogicalName(), n.natGateways)
}
