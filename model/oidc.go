package model

type OIDC struct {
	Enabled       bool   `yaml:"enabled"`
	IssuerURL     string `yaml:"issuerUrl"`
	ClientID      string `yaml:"clientId"`
	UsernameClaim string `yaml:"usernameClaim"`
	GroupsClaim   string `yaml:"groupsClaim,omitempty"`
	SelfSignedCa  bool   `yaml:"selfSignedCa"`
}
