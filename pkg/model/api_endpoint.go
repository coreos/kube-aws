package model

import "github.com/kubernetes-incubator/kube-aws/pkg/api"

// APIEndpoint represents a Kubernetes API endpoint
type APIEndpoint struct {
	// APIEndpoint derives the user-provided configuration in an item of an `apiEndpoints` array and adds various computed settings
	api.APIEndpoint
	// LoadBalancer is the load balancer serving this API endpoint if any
	LoadBalancer APIEndpointLB
}
