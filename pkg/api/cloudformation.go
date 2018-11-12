package api

type CloudFormation struct {
	RoleARN            string             `yaml:"roleARN,omitempty"`
	StackNameOverrides StackNameOverrides `yaml:"stackNameOverrides,omitempty"`
}
