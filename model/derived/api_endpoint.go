package derived

import "github.com/kubernetes-incubator/kube-aws/model"

// APIEndpoint represents a Kubernetes API endpoint
type APIEndpoint struct {
	// APIEndpoint derives the user-provided configuration in an item of an `apiEndpoints` array and adds various computed settings
	model.APIEndpoint
	// LoadBalancer is the load balancer serving this API endpoint if any
	LoadBalancer APIEndpointLB
}
