package api

// SubnetReference references one of subnets defined in the top-level of cluster.yaml
type SubnetReference struct {
	// Name is the unique name of subnet to be referenced.
	// The subnet referenced by this name should be defined in the `subnets[]` field in the top-level of cluster.yaml
	Name string `yaml:"name,omitempty"`
}
