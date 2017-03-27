package model

import (
	"fmt"
)

// DefaultAPIEndpointName returns the default endpoint name used when you've omitted the `name` key in each item of the `apiEndpintsp[]` array
const DefaultAPIEndpointName = "Default"

// NewDefaultAPIEndpoints creates the slice of API endpoints containing only the default one which is with arbitrary DNS name and an ELB
func NewDefaultAPIEndpoints(dnsName string, subnets []SubnetReference, hostedZoneId string, createRecordSet bool, private bool) []APIEndpoint {
	return []APIEndpoint{
		APIEndpoint{
			Name:    DefaultAPIEndpointName,
			DNSName: dnsName,
			LoadBalancer: APIEndpointLB{
				SubnetReferences: subnets,
				HostedZone: HostedZone{
					Identifier: Identifier{
						ID: hostedZoneId,
					},
				},
				CreateRecordSet:  &createRecordSet,
				PrivateSpecified: &private,
			},
		},
	}
}

// APIEndpoint is a Kubernetes API endpoint to which various clients connect.
// Each endpoint can be served by an existing ELB or a kube-aws managed ELB.
type APIEndpoint struct {
	// Name is the unique name of this API endpoint used by kube-aws for identifying this API endpoint
	Name string `yaml:"name,omitempty"`
	// DNSName is the FQDN of this endpoint
	// A record set may or may not be created with this DNS name.
	// TLS certificates generated by kube-aws would contain this name in the list of common names.
	DNSName string `yaml:"dnsName,omitempty"`
	// LoadBalancer is a set of an ELB and relevant settings and resources to serve a Kubernetes API hosted by controller nodes
	LoadBalancer APIEndpointLB `yaml:"loadBalancer,omitempty"`
	//DNSRoundRobin APIDNSRoundRobin `yaml:"dnsRoundRobin,omitempty"`
}

// Validate returns an error when there's any user error in the `apiEndpoint` settings
func (e APIEndpoint) Validate() error {
	if err := e.LoadBalancer.Validate(); err != nil {
		return fmt.Errorf("invalid apiEndpoint named \"%s\": invalid loadBalancer: %v", e.Name, err)
	}
	return nil
}

//type APIDNSRoundRobin struct {
//	// PrivateSpecified determines the resulting DNS round robin uses private IPs of the nodes for an endpoint
//	PrivateSpecified bool
//	// HostedZone is where the resulting A records are created for an endpoint
//      // Beware that kube-aws will never create a hosted zone used for a DNS round-robin because
//      // Doing so would result in CloudFormation to be unable to remove the hosted zone when the stack is deleted
//	HostedZone HostedZone
//}
