package api

// kube-aws manages at most one VPC per cluster
// If ID or IDFromStackOutput is non-zero, kube-aws doesn't manage the VPC but its users' responsibility to
// provide properly configured one to be reused by kube-aws.
// More concretely:
// * If an user is going to reuse an existing VPC, it must have an internet gateway attached and
// * A valid internet gateway ID must be provided via `internetGateway.id` or `internetGateway.idFromStackOutput`.
//   In other words, kube-aws doesn't create an internet gateway in an existing VPC.
type VPC struct {
	Identifier `yaml:",inline"`
}

func (v VPC) ImportFromNetworkStack() VPC {
	if !v.HasIdentifier() {
		// Otherwise import the VPC ID from the control-plane stack
		v.IDFromFn = `{"Fn::ImportValue":{"Fn::Sub":"${NetworkStackName}-VPC"}}`
	}
	return v
}
