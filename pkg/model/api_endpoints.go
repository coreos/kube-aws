package model

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"sort"
	"strings"
)

// APIEndpoints is a set of API endpoints associated to a Kubernetes cluster
type APIEndpoints map[string]APIEndpoint

// NewAPIEndpoints computes and returns all the required settings required to manage API endpoints form various user-inputs and other already-computed settings
func NewAPIEndpoints(configs []api.APIEndpoint, allSubnets []api.Subnet) (APIEndpoints, error) {
	endpoints := map[string]APIEndpoint{}

	findSubnetByReference := func(ref api.SubnetReference) (*api.Subnet, error) {
		for _, s := range allSubnets {
			if s.Name == ref.Name {
				return &s, nil
			}
		}
		return nil, fmt.Errorf("no subnets named %s found in cluster.yaml", ref.Name)
	}

	findSubnetsByReferences := func(refs []api.SubnetReference) ([]api.Subnet, error) {
		subnets := []api.Subnet{}
		for i, r := range refs {
			s, err := findSubnetByReference(r)
			if err != nil {
				return []api.Subnet{}, fmt.Errorf("error in subnet ref at index %d: %v", i, err)
			}
			subnets = append(subnets, *s)
		}
		return subnets, nil
	}

	for i, c := range configs {
		var err error
		var lbSubnets []api.Subnet
		lbConfig := c.LoadBalancer
		if lbConfig.ManageELB() {
			if len(lbConfig.SubnetReferences) > 0 {
				lbSubnets, err = findSubnetsByReferences(lbConfig.SubnetReferences)
				if err != nil {
					return nil, fmt.Errorf("invalid api endpint config at index %d: %v", i, err)
				}
			} else {
				for _, s := range allSubnets {
					if s.Private == lbConfig.Private() {
						lbSubnets = append(lbSubnets, s)
					}
				}
				if len(lbSubnets) == 0 {
					return nil, fmt.Errorf("invalid api endpoint config at index %d: no appropriate subnets found for api load balancer with private=%v", i, lbConfig.PrivateSpecified)
				}
			}
		}
		endpoint := APIEndpoint{
			APIEndpoint: c,
			LoadBalancer: APIEndpointLB{
				APIEndpointLB: lbConfig,
				APIEndpoint:   c,
				Subnets:       lbSubnets,
			},
		}
		if _, exists := endpoints[c.Name]; exists {
			return nil, fmt.Errorf("invalid api endpoint config at index %d: api endpint named %s already exists", i, c.Name)
		}
		endpoints[c.Name] = endpoint
	}

	return endpoints, nil
}

// FindByName finds an API endpoint in this set by its name
func (e APIEndpoints) FindByName(name string) (*APIEndpoint, error) {
	endpoint, exists := e[name]
	if exists {
		return &endpoint, nil
	}

	apiEndpointNames := []string{}
	for _, endpoint := range e {
		apiEndpointNames = append(apiEndpointNames, endpoint.Name)
	}

	return nil, fmt.Errorf("no API endpoint named \"%s\" defined under the `apiEndpoints[]`. The name must be one of: %s", name, strings.Join(apiEndpointNames, ", "))
}

// ELBClassicRefs returns the names of all the Classic ELBs to which controller nodes should be associated
func (e APIEndpoints) ELBClassicRefs() []string {
	refs := []string{}
	for _, endpoint := range e {
		if endpoint.LoadBalancer.Enabled() && endpoint.LoadBalancer.ClassicLoadBalancer() {
			refs = append(refs, endpoint.LoadBalancer.Ref())
		}
	}
	return refs
}

// ELBV2TargetGroupRefs returns the names of all the Load Balancers v2 to which controller nodes should be associated
func (e APIEndpoints) ELBV2TargetGroupRefs() []string {
	refs := []string{}
	for _, endpoint := range e {
		if endpoint.LoadBalancer.Enabled() && endpoint.LoadBalancer.LoadBalancerV2() {
			refs = append(refs, endpoint.LoadBalancer.TargetGroupRef())
		}
	}
	return refs
}

// ManageELBLogicalNames returns all the logical names of the cfn resources corresponding to ELBs managed by kube-aws for API endpoints
func (e APIEndpoints) ManagedELBLogicalNames() []string {
	logicalNames := []string{}
	for _, endpoint := range e {
		if endpoint.LoadBalancer.ManageELB() {
			logicalNames = append(logicalNames, endpoint.LoadBalancer.LogicalName())
		}
	}
	sort.Strings(logicalNames)
	return logicalNames
}

// GetDefault returns the default API endpoint identified by its name.
// The name is defined as DefaultAPIEndpointName
func (e APIEndpoints) GetDefault() APIEndpoint {
	if len(e) != 1 {
		panic(fmt.Sprintf("[bug] GetDefault invoked with an unexpected number of API endpoints: %d", len(e)))
	}
	var name string
	for n, _ := range e {
		name = n
		break
	}
	return e[name]
}
