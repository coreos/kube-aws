package model

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
	// Classic is set to true for provisioning an ELB, false for an ALB
	// TODO Default to true
	Classic bool `yaml:"classic,omitempty"`
	// CertificateARN is the ARN of an existing TLS certificate
	CertificateARN string `yaml:"certificateArn,omitempty"`
	// CreateRecordSet is set to false when you want to disable creation of the record set for this api load balancer
	CreateRecordSet *bool `yaml:"createRecordSet,omitempty"`
	// Identifier specifies an existing load-balancer used for load-balancing controller nodes and serving this endpoint
	Identifier Identifier `yaml:",inline"`
	// Subnets contains all the subnets assigned to this load-balancer. Specified only when this load balancer is not reused but managed one
	SubnetReferences []SubnetReference `yaml:"subnets,omitempty"`
	// PrivateSpecified determines the resulting load balancer uses an internal elb for an endpoint
	PrivateSpecified *bool `yaml:"private,omitempty"`
	// RecordSetTTLSpecified is the TTL for the record set to this load balancer. Defaults to 300 if nil
	RecordSetTTLSpecified *int `yaml:"recordSetTTL,omitempty"`
	// HostedZone is where the resulting Alias record is created for an endpoint
	HostedZone HostedZone `yaml:"hostedZone,omitempty"`
	//// SecurityGroups contains extra security groups must be associated to the lb serving API requests from clients
	//SecurityGroups []SecurityGroup
	// SecurityGroupIds represents SGs associated to this LB. Required when APIAccessAllowedSourceCIDRs is explicitly set to empty
	SecurityGroupIds []string `yaml:"securityGroupIds"`
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
	return e.ManageELBRecordSet() || e.CreateRecordSet != nil
}

// ManageELBRecordSet returns tru if kube-aws should create a record set for the ELB
func (e APIEndpointLB) ManageELBRecordSet() bool {
	return e.HostedZone.HasIdentifier() && (e.CreateRecordSet == nil || (e.CreateRecordSet != nil && *e.CreateRecordSet))
}

// ManageSecurityGroup returns true if kube-aws should create a security group for this ELB
func (e APIEndpointLB) ManageSecurityGroup() bool {
	return len(e.APIAccessAllowedSourceCIDRs) > 0
}

// Validate returns an error when there's any user error in the settings of the `loadBalancer` field
func (e APIEndpointLB) Validate() error {
	if e.managedRecordSetImplied() && !e.HostedZone.HasIdentifier() {
		return errors.New("missing hostedZoneId")
	}
	if e.Identifier.HasIdentifier() && (e.PrivateSpecified != nil || len(e.SubnetReferences) > 0 || e.CreateRecordSet != nil || e.HostedZone.HasIdentifier()) {
		return errors.New("createRecordSet, private, subnets, hostedZone must be omitted when id is specified to reuse an existing ELB")
	}
	if e.RecordSetTTLSpecified != nil && *e.RecordSetTTLSpecified < 1 {
		return errors.New("recordSetTTL must be at least 1 second")
	}
	if e.managedELBImplied() && len(e.APIAccessAllowedSourceCIDRs) == 0 && len(e.SecurityGroupIds) == 0 {
		return errors.New("either apiAccessAllowedSourceCIDRs or securityGroupIds must be present. Try not to explicitly empty apiAccessAllowedSourceCIDRs or set one or more securityGroupIDs")
	}
	return nil
}

func (e APIEndpointLB) managedRecordSetImplied() bool {
	return (e.CreateRecordSet == nil && e.managedELBImplied()) || (e.CreateRecordSet != nil && *e.CreateRecordSet)
}

func (e APIEndpointLB) managedELBImplied() bool {
	return len(e.SubnetReferences) > 0 || e.explicitlyPrivate() || e.explicitlyPublic()
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
