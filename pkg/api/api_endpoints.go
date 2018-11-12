package api

import (
	"fmt"
)

type APIEndpoints []APIEndpoint

const (
	// DefaultAPIEndpointName is the default endpoint name used when you've omitted `apiEndpoints` but not `externalDNSName`
	DefaultAPIEndpointName = "Default"

	// DefaultLoadBalancerType is the default load balancer to be provisioned by kube-aws for the API endpoints
	DefaultLoadBalancerType = "classic"
)

// NewDefaultAPIEndpoints creates the slice of API endpoints containing only the default one which is with arbitrary DNS name and an ELB
func NewDefaultAPIEndpoints(dnsName string, subnets []SubnetReference, hostedZoneId string, recordSetTTL int, private bool) APIEndpoints {
	defaultLBType := DefaultLoadBalancerType

	return []APIEndpoint{
		APIEndpoint{
			Name:    DefaultAPIEndpointName,
			DNSName: dnsName,
			LoadBalancer: APIEndpointLB{
				Type:                        &defaultLBType,
				APIAccessAllowedSourceCIDRs: DefaultCIDRRanges(),
				SubnetReferences:            subnets,
				HostedZone: HostedZone{
					Identifier: Identifier{
						ID: hostedZoneId,
					},
				},
				RecordSetTTLSpecified: &recordSetTTL,
				PrivateSpecified:      &private,
			},
		},
	}
}

// Validate returns an error if there's any user error in the settings of apiEndpoints
func (e APIEndpoints) Validate() error {
	for i, apiEndpoint := range e {
		if err := apiEndpoint.Validate(); err != nil {
			return fmt.Errorf("invalid apiEndpoint \"%s\" at index %d: %v", apiEndpoint.Name, i, err)
		}
	}
	return nil
}

// HasNetworkLoadBalancers returns true if there's any API endpoint load balancer of type 'network'
func (e APIEndpoints) HasNetworkLoadBalancers() bool {
	for _, apiEndpoint := range e {
		if apiEndpoint.LoadBalancer.NetworkLoadBalancer() {
			return true
		}
	}
	return false
}

//type APIDNSRoundRobin struct {
//	// PrivateSpecified determines the resulting DNS round robin uses private IPs of the nodes for an endpoint
//	PrivateSpecified bool
//	// HostedZone is where the resulting A records are created for an endpoint
//      // Beware that kube-aws will never create a hosted zone used for a DNS round-robin because
//      // Doing so would result in CloudFormation to be unable to remove the hosted zone when the stack is deleted
//	HostedZone HostedZone
//}
