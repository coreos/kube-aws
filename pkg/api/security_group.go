package api

// SecurityGroup references one of existing security groups in your AWS account
type SecurityGroup struct {
	Identifier `yaml:",inline"`
}
