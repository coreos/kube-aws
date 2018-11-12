package api

import (
	"errors"
	"fmt"
)

// DefaultRecordSetTTL is the default value for the loadBalancer.recordSetTTL key
const DefaultRecordSetTTL = 300

// APIEndpointLB is a set of an ELB and relevant settings and resources to serve a Kubernetes API hosted by controller nodes
type APIEndpointLB struct {
	// APIAccessAllowedSourceCIDRs is network ranges of sources you'd like Kubernetes API accesses to be allowed from, in CIDR notation
	APIAccessAllowedSourceCIDRs CIDRRanges `yaml:"apiAccessAllowedSourceCIDRs,omitempty"`
	// Identifier specifies an existing load-balancer used for load-balancing controller nodes and serving this endpoint
	Identifier Identifier `yaml:",inline"`
	// Managed is set to true when want to create an ELB for this API endpoint. It is false by default i.e. considered to be false if nil
	Managed *bool `yaml:"managed,omitempty"`
	// Subnets contains all the subnets assigned to this load-balancer. Specified only when this load balancer is not reused but managed one
	SubnetReferences []SubnetReference `yaml:"subnets,omitempty"`
	// PrivateSpecified determines the resulting load balancer uses an internal elb for an endpoint
	PrivateSpecified *bool `yaml:"private,omitempty"`
	// RecordSetManaged represents if the user wants kube-aws not to create a record set for this API load balancer
	// i.e. the user wants to configure Route53 or one's own DNS oneself
	RecordSetManaged *bool `yaml:"recordSetManaged,omitempty"`
	// RecordSetTTLSpecified is the TTL for the record set to this load balancer. Defaults to 300 if nil
	RecordSetTTLSpecified *int `yaml:"recordSetTTL,omitempty"`
	// HostedZone is where the resulting Alias record is created for an endpoint
	HostedZone HostedZone `yaml:"hostedZone,omitempty"`
	//// SecurityGroups contains extra security groups must be associated to the lb serving API requests from clients
	//SecurityGroups []SecurityGroup
	// SecurityGroupIds represents SGs associated to this LB. Required when APIAccessAllowedSourceCIDRs is explicitly set to empty
	SecurityGroupIds []string `yaml:"securityGroupIds"`
	// Load balancer type. It is 'classic' by default, but can be changed to 'network'
	Type *string `yaml:"type,omitempty"`
}

// UnmarshalYAML unmarshals YAML data to an APIEndpointLB object with defaults
// This doesn't work due to a go-yaml issue described in http://ghodss.com/2014/the-right-way-to-handle-yaml-in-golang/
// And that's why we need to implement `func (e APIEndpointLB) RecordSetTTL() int` for defaulting.
// TODO Migrate to ghodss/yaml
func (e *APIEndpointLB) UnmarshalYAML(unmarshal func(interface{}) error) error {
	ttl := DefaultRecordSetTTL
	type t APIEndpointLB
	work := t(APIEndpointLB{
		RecordSetTTLSpecified:       &ttl,
		APIAccessAllowedSourceCIDRs: DefaultCIDRRanges(),
	})
	if err := unmarshal(&work); err != nil {
		return fmt.Errorf("failed to parse API endpoint LB config: %v", err)
	}
	*e = APIEndpointLB(work)
	return nil
}

// ManageELB returns true if an ELB should be managed by kube-aws
func (e APIEndpointLB) ManageELB() bool {
	return e.managedELBImplied() || (e.Managed != nil && *e.Managed)
}

// ClassicLoadBalancer returns true if the load balancer is a classic ELB
func (e APIEndpointLB) ClassicLoadBalancer() bool {
	return e.Type == nil || *e.Type == "classic"
}

// LoadBalancerV2 returns true if the load balancer is a ELBV2 load balancer (only network load balancer is supported for now)
func (e APIEndpointLB) LoadBalancerV2() bool {
	return e.Type != nil && *e.Type != "classic"
}

// NetworkLoadBalancer returns true if the load balancer is a ELBV2 network load balancer
func (e APIEndpointLB) NetworkLoadBalancer() bool {
	return e.Type != nil && *e.Type == "network"
}

// ManageELBRecordSet returns true if kube-aws should create a record set for the ELB
func (e APIEndpointLB) ManageELBRecordSet() bool {
	return e.HostedZone.HasIdentifier()
}

// ManageSecurityGroup returns true if kube-aws should create a security group for this ELB
func (e APIEndpointLB) ManageSecurityGroup() bool {
	return !e.NetworkLoadBalancer() && len(e.APIAccessAllowedSourceCIDRs) > 0
}

// Validate returns an error when there's any user error in the settings of the `loadBalancer` field
func (e APIEndpointLB) Validate() error {
	if e.Identifier.HasIdentifier() {
		if e.PrivateSpecified != nil || !e.ClassicLoadBalancer() || len(e.SubnetReferences) > 0 || e.HostedZone.HasIdentifier() {
			return errors.New("type, private, subnets, hostedZone must be omitted when id is specified to reuse an existing ELB")
		}

		return nil
	}

	if e.Managed != nil && !*e.Managed {
		if e.RecordSetTTL() != DefaultRecordSetTTL {
			return errors.New("recordSetTTL should not be modified when an API endpoint LB is not managed by kube-aws")
		}

		if e.HostedZone.HasIdentifier() {
			return errors.New("hostedZone.id should not be specified when an API endpoint LB is not managed by kube-aws")
		}

		if e.Type != nil && len(*e.Type) > 0 {
			return errors.New("type should not be specified when an API endpoint LB is not managed by kube-aws")
		}

		return nil
	}

	if e.HostedZone.HasIdentifier() {
		if e.RecordSetManaged != nil && !*e.RecordSetManaged {
			return errors.New("hostedZone.id must be omitted when you want kube-aws not to touch Route53")
		}

		if e.RecordSetTTL() < 1 {
			return errors.New("recordSetTTL must be at least 1 second")
		}
	} else {
		if e.RecordSetManaged == nil || *e.RecordSetManaged {
			return errors.New("missing hostedZone.id: hostedZone.id is required when `recordSetManaged` is set to true. If you do want to configure DNS yourself, set it to true")
		}

		if e.RecordSetTTL() != DefaultRecordSetTTL {
			return errors.New(
				"recordSetTTL should not be modified when hostedZone id is nil",
			)
		}
	}

	if e.ClassicLoadBalancer() && e.ManageELB() && len(e.APIAccessAllowedSourceCIDRs) == 0 && len(e.SecurityGroupIds) == 0 {
		return errors.New("either apiAccessAllowedSourceCIDRs or securityGroupIds must be present. Try not to explicitly empty apiAccessAllowedSourceCIDRs or set one or more securityGroupIDs")
	}

	if !e.NetworkLoadBalancer() && !e.ClassicLoadBalancer() {
		return errors.New("load balancer type must be either 'classic' or 'network'")
	}

	if e.NetworkLoadBalancer() {
		if len(e.SecurityGroupIds) > 0 {
			return errors.New("cannot specify security group IDs for a network load balancer")
		}
	}

	return nil
}

func (e APIEndpointLB) managedELBImplied() bool {
	return len(e.SubnetReferences) > 0 ||
		e.explicitlyPrivate() ||
		e.explicitlyPublic() ||
		e.HostedZone.HasIdentifier() ||
		len(e.SecurityGroupIds) > 0 ||
		e.RecordSetManaged != nil
}

func (e APIEndpointLB) explicitlyPrivate() bool {
	return e.PrivateSpecified != nil && *e.PrivateSpecified
}

func (e APIEndpointLB) explicitlyPublic() bool {
	return e.PrivateSpecified != nil && !*e.PrivateSpecified
}

// RecordSetTTL is the TTL for the record set to this load balancer. Defaults to 300 if `recordSetTTL` is omitted/set to nil
func (e APIEndpointLB) RecordSetTTL() int {
	if e.RecordSetTTLSpecified != nil {
		return *e.RecordSetTTLSpecified
	}
	return DefaultRecordSetTTL
}

// Private returns true when this LB is a private one i.e. the `private` field is explicitly set to true
func (e APIEndpointLB) Private() bool {
	return e.explicitlyPrivate()
}
