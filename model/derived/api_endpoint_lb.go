package derived

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/model"
	"strings"
)

// APIEndpointLB is the load balancer serving the API endpoint
type APIEndpointLB struct {
	// APIEndpointLB derives the user-provided configuration in an `apiEndpoints[].loadBalancer` and adds various computed settings
	model.APIEndpointLB
	// APIEndpoint is inherited to configure this load balancer
	model.APIEndpoint
	// Subnets contains all the subnets assigned to this load-balancer. Specified only when this load balancer is not reused but managed one
	Subnets []model.Subnet
}

// DNSNameRef returns a CloudFormation ref for the Amazon-provided DNS name of this load balancer, which is typically used
// to fill an ALIAS or a CNAME dns record in Route 53
func (b APIEndpointLB) DNSNameRef() string {
	return fmt.Sprintf(`{ "Fn::GetAtt": ["%s", "DNSName"]}`, b.LogicalName())
}

// RecordSetLogicalName returns the logical name of a record set created for this load balancer
// A logical name is an unique name of an AWS resource inside a CloudFormation stack template
func (b APIEndpointLB) RecordSetLogicalName() string {
	return fmt.Sprintf("%sRecordSet", b.LogicalName())
}

// HostedZoneRef returns a CloudFormation ref for the hosted zone the record set for this load balancer is created in
func (b APIEndpointLB) HostedZoneRef() string {
	return b.HostedZone.Identifier.ID
}

// LogicalName returns the unique resource name of the ELB
func (b APIEndpointLB) LogicalName() string {
	return fmt.Sprintf("APIEndpoint%sELB", strings.Title(b.Name))
}

// Enabled returns true when controller nodes should be added as targets of this load balancer
func (b APIEndpointLB) Enabled() bool {
	return b.ManageELB() || b.Identifier.HasIdentifier()
}

// Ref returns a CloudFormation ref for the ELB backing the API endpoint
func (b APIEndpointLB) Ref() string {
	return b.Identifier.Ref(func() string {
		return b.LogicalName()
	})
}

func (b APIEndpointLB) SecurityGroupLogicalName() string {
	return fmt.Sprintf("APIEndpoint%sSG", strings.Title(b.Name))
}

func (b APIEndpointLB) TargetGroupLogicalName() string {
	return fmt.Sprintf("APITG%s", b.Name)
}

func (b APIEndpointLB) TargetGroupARNRef() string {
	return fmt.Sprintf(`{"Ref":"%s"}`, b.TargetGroupLogicalName())
}

// SecurityGroupRefs contains CloudFormation resource references for additional SGs associated to this LB
func (b APIEndpointLB) SecurityGroupRefs() []string {
	refs := []string{}

	for _, id := range b.SecurityGroupIds {
		refs = append(refs, fmt.Sprintf(`"%s"`, id))
	}

	if b.ManageSecurityGroup() {
		refs = append(refs, fmt.Sprintf(`{"Ref":"%s"}`, b.SecurityGroupLogicalName()))
	}

	refs = append(
		refs,
		`{"Ref":"SecurityGroupElbAPIServer"}`,
	)

	return refs
}
