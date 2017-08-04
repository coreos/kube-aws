package model

import (
	"net/url"
)

type Oidc struct {
	Enabled      bool   `yaml:"enabled"`
	Url          string `yaml:"url"`
	ClientId     string `yaml:"clientId"`
	Username     string `yaml:"username"`
	Groups       string `yaml:"groups,omitempty"`
	SelfSignedCa bool   `yaml:"selfSignedCa"`
}

func (c Oidc) OidcDNSNames() string {
	u, _ := url.Parse(c.Url)
	return u.Host
}
