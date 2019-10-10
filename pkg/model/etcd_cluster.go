package model

import (
	"fmt"
	"regexp"

	"github.com/kubernetes-incubator/kube-aws/pkg/api"
)

type EtcdCluster struct {
	api.EtcdCluster
	Network
	region    api.Region
	nodeCount int
}

func NewEtcdCluster(config api.EtcdCluster, region api.Region, network Network, nodeCount int) EtcdCluster {
	return EtcdCluster{
		EtcdCluster: config,
		region:      region,
		Network:     network,
		nodeCount:   nodeCount,
	}
}

func (c EtcdCluster) Region() api.Region {
	return c.region
}

func (c EtcdCluster) NodeCount() int {
	return c.nodeCount
}

func (c EtcdCluster) DNSNames() []string {
	var dnsName string
	if c.GetMemberIdentityProvider() == api.MemberIdentityProviderEIP {
		// Used when `etcd.memberIdentityProvider` is set to "eip"
		dnsName = fmt.Sprintf("*.%s", c.region.PublicComputeDomainName())
	}
	if c.GetMemberIdentityProvider() == api.MemberIdentityProviderENI {
		if c.InternalDomainName != "" {
			// Used when `etcd.memberIdentityProvider` is set to "eni" with non-empty `etcd.internalDomainName`
			dnsName = fmt.Sprintf("*.%s", c.InternalDomainName)
		} else {
			dnsName = fmt.Sprintf("*.%s", c.region.PrivateDomainName())
		}
	}
	return []string{dnsName}
}

func (c EtcdCluster) LogicalName() string {
	d := regexp.MustCompile(`\.`)
	return fmt.Sprintf("Etcd%s", d.ReplaceAllString(c.EtcdCluster.MajorMinorVersion(), `dot`))
}
