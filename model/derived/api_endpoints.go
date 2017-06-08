package derived

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/model"
	"sort"
	"strings"
)

// APIEndpoints is a set of API endpoints associated to a Kubernetes cluster
type APIEndpoints map[string]APIEndpoint

// NewAPIEndpoints computes and returns all the required settings required to manage API endpoints form various user-inputs and other already-computed settings
func NewAPIEndpoints(configs []model.APIEndpoint, allSubnets []model.Subnet) (APIEndpoints, error) {
	endpoints := map[string]APIEndpoint{}

	findSubnetByReference := func(ref model.SubnetReference) (*model.Subnet, error) {
		for _, s := range allSubnets {
			if s.Name == ref.Name {
				return &s, nil
			}
		}
		return nil, fmt.Errorf("no subnets named %s found in cluster.yaml", ref.Name)
	}

	findSubnetsByReferences := func(refs []model.SubnetReference) ([]model.Subnet, error) {
		subnets := []model.Subnet{}
		for i, r := range refs {
			s, err := findSubnetByReference(r)
			if err != nil {
				return []model.Subnet{}, fmt.Errorf("error in subnet ref at index %d: %v", i, err)
			}
			subnets = append(subnets, *s)
		}
		return subnets, nil
	}

	for i, c := range configs {
		var err error
		var lbSubnets []model.Subnet
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

// ELBRefs returns the names of all the ELBs to which controller nodes should be associated
func (e APIEndpoints) ELBRefs() []string {
	refs := []string{}
	for _, endpoint := range e {
		lb := endpoint.LoadBalancer
		if lb.Enabled() && lb.Classic {
			refs = append(refs, lb.Ref())
		}
	}
	return refs
}

// TargetGroupARNRefs returns the names of all the ALB target groups to which controller nodes should be associated
func (e APIEndpoints) TargetGroupARNRefs() []string {
	refs := []string{}
	for _, endpoint := range e {
		lb := endpoint.LoadBalancer
		if lb.Enabled() && !lb.Classic {
			refs = append(refs, lb.TargetGroupARNRef())
		}
	}
	return refs
}

// ManageELBLogicalNames returns all the logical names of the cfn resources corresponding to ELBs managed by kube-aws for API endpoints
func (e APIEndpoints) ManagedELBLogicalNames() []string {
	logicalNames := []string{}
	for _, endpoint := range e {
		lb := endpoint.LoadBalancer
		if lb.ManageELB() && lb.Classic {
			logicalNames = append(logicalNames, lb.LogicalName())
		}
	}
	sort.Strings(logicalNames)
	return logicalNames
}

// ManageALBLogicalNames returns all the logical names of the cfn resources corresponding to ALBs managed by kube-aws for API endpoints
func (e APIEndpoints) ManagedALBLogicalNames() []string {
	logicalNames := []string{}
	for _, endpoint := range e {
		lb := endpoint.LoadBalancer
		if lb.ManageELB() && !lb.Classic {
			logicalNames = append(logicalNames, lb.LogicalName())
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
