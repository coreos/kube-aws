package derived

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/model"
)

type EtcdCluster struct {
	model.EtcdCluster
	Network
	region    model.Region
	nodeCount int
}

func NewEtcdCluster(config model.EtcdCluster, region model.Region, network Network, nodeCount int) EtcdCluster {
	return EtcdCluster{
		EtcdCluster: config,
		region:      region,
		Network:     network,
		nodeCount:   nodeCount,
	}
}

func (c EtcdCluster) Region() model.Region {
	return c.region
}

func (c EtcdCluster) NodeCount() int {
	return c.nodeCount
}

func (c EtcdCluster) DNSNames() []string {
	var dnsName string
	if c.GetMemberIdentityProvider() == model.MemberIdentityProviderEIP {
		// Used when `etcd.memberIdentityProvider` is set to "eip"
		dnsName = fmt.Sprintf("*.%s", c.region.PublicComputeDomainName())
	}
	if c.GetMemberIdentityProvider() == model.MemberIdentityProviderENI {
		if c.InternalDomainName != "" {
			// Used when `etcd.memberIdentityProvider` is set to "eni" with non-empty `etcd.internalDomainName`
			dnsName = fmt.Sprintf("*.%s", c.InternalDomainName)
		} else {
			dnsName = fmt.Sprintf("*.%s", c.region.PrivateDomainName())
		}
	}
	return []string{dnsName}
}
