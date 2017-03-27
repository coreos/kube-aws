package model

import "errors"

// APIEndpointLB is a set of an ELB and relevant settings and resources to serve a Kubernetes API hosted by controller nodes
type APIEndpointLB struct {
	// CreateRecordSet is set to false when you want to disable creation of the record set for this api load balancer
	CreateRecordSet *bool `yaml:"createRecordSet,omitempty"`
	// Identifier specifies an existing load-balancer used for load-balancing controller nodes and serving this endpoint
	Identifier Identifier `yaml:",inline"`
	// Subnets contains all the subnets assigned to this load-balancer. Specified only when this load balancer is not reused but managed one
	SubnetReferences []SubnetReference `yaml:"subnets,omitempty"`
	// PrivateSpecified determines the resulting load balancer uses an internal elb for an endpoint
	PrivateSpecified *bool `yaml:"private,omitempty"`
	// HostedZone is where the resulting Alias record is created for an endpoint
	HostedZone HostedZone `yaml:"hostedZone,omitempty"`
	//// SecurityGroups contains extra security groups must be associated to the lb serving API requests from clients
	//SecurityGroups []SecurityGroup
}

// ManageELB returns true if an ELB should be managed by kube-aws
func (e APIEndpointLB) ManageELB() bool {
	return e.ManageELBRecordSet() || e.CreateRecordSet != nil
}

// ManageELBRecordSet returns tru if kube-aws should create a record set for the ELB
func (e APIEndpointLB) ManageELBRecordSet() bool {
	return e.HostedZone.HasIdentifier() && (e.CreateRecordSet == nil || (e.CreateRecordSet != nil && *e.CreateRecordSet))
}

// Validate returns an error when there's any user error in the settings of the `loadBalancer` field
func (e APIEndpointLB) Validate() error {
	if e.managedRecordSetImplied() && !e.HostedZone.HasIdentifier() {
		return errors.New("missing hostedZoneId")
	}
	if e.Identifier.HasIdentifier() && (e.PrivateSpecified != nil || len(e.SubnetReferences) > 0 || e.CreateRecordSet != nil || e.HostedZone.HasIdentifier()) {
		return errors.New("createRecordSet, private, subnets, hostedZone must be omitted when id is specified to reuse an existing ELB")
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

// Private returns true when this LB is a private one i.e. the `private` field is explicitly set to true
func (e APIEndpointLB) Private() bool {
	return e.explicitlyPrivate()
}
