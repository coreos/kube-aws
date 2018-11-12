package api

var (
	// ControlPlaneStackName is the logical name of a CloudFormation stack resource in a root stack template
	// This is not needed to be unique in an AWS account because the actual name of a nested stack is generated randomly
	// by CloudFormation by including the logical name.
	// This is NOT intended to be used to reference stack name from cloud-config as the target of awscli or cfn-bootstrap-tools commands e.g. `cfn-init` and `cfn-signal`
	controlPlaneStackName = "control-plane"
)
