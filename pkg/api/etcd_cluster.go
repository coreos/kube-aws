package api

import "fmt"

type EtcdCluster struct {
	InternalDomainName     string      `yaml:"internalDomainName,omitempty"`
	MemberIdentityProvider string      `yaml:"memberIdentityProvider,omitempty"`
	HostedZone             Identifier  `yaml:"hostedZone,omitempty"`
	ManageRecordSets       *bool       `yaml:"manageRecordSets,omitempty"`
	KMSKeyARN              string      `yaml:"kmsKeyArn,omitempty"`
	Version                EtcdVersion `yaml:"version,omitempty"`
}

const (
	MemberIdentityProviderEIP = "eip"
	MemberIdentityProviderENI = "eni"
)

func (c EtcdCluster) EC2InternalDomainUsed() bool {
	return c.InternalDomainName == ""
}

func (c EtcdCluster) GetMemberIdentityProvider() string {
	p := c.MemberIdentityProvider

	if p == MemberIdentityProviderEIP || p == MemberIdentityProviderENI {
		return p
	} else if p == "" {
		return MemberIdentityProviderEIP
	}

	panic(fmt.Errorf("Unsupported memberIdentityProvider: %s", p))
}

func (e EtcdCluster) hostedZoneManaged() bool {
	return e.GetMemberIdentityProvider() == MemberIdentityProviderENI &&
		!e.HostedZone.HasIdentifier() && !e.EC2InternalDomainUsed()
}

// Notes:
// * EC2's default domain like <region>.compute.internal for internalDomainName implies not to manage record sets
// * Managed hosted zone implies managed record sets
func (e EtcdCluster) RecordSetsManaged() bool {
	return e.GetMemberIdentityProvider() == MemberIdentityProviderENI && !e.EC2InternalDomainUsed() &&
		(e.hostedZoneManaged() || (e.ManageRecordSets == nil || *e.ManageRecordSets))
}

// NodeShouldHaveSecondaryENI returns true if all the etcd nodes should have secondary ENIs for their identities
func (c EtcdCluster) NodeShouldHaveSecondaryENI() bool {
	return c.GetMemberIdentityProvider() == MemberIdentityProviderENI
}

// NodeShouldHaveEIP returns true if all the etcd nodes should have EIPs for their identities
func (c EtcdCluster) NodeShouldHaveEIP() bool {
	return c.GetMemberIdentityProvider() == MemberIdentityProviderEIP
}
