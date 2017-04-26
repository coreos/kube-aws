package model

import (
	"net/url"
)

type Dex struct {
	Enabled         bool             `yaml:"enabled"`
	Url             string           `yaml:"url"`
	ClientId        string           `yaml:"clientId"`
	Username        string           `yaml:"username"`
	Groups          string           `yaml:"groups,omitempty"`
	SelfSignedCa    bool             `yaml:"selfSignedCa"`
	Connectors      []Connector      `yaml:"connectors,omitempty"`
	StaticClients   []StaticClient   `yaml:"staticClients,omitempty"`
	StaticPasswords []StaticPassword `yaml:"staticPasswords,omitempty"`
}

type Connector struct {
	Type   string            `yaml:"type"`
	Id     string            `yaml:"id"`
	Name   string            `yaml:"name"`
	Config map[string]string `yaml:"config"`
}

type StaticClient struct {
	Id           string `yaml:"id"`
	RedirectURIs string `yaml:"redirectURIs"`
	Name         string `yaml:"name"`
	Secret       string `yaml:"secret"`
}

type StaticPassword struct {
	Email    string `yaml:"email"`
	Hash     string `yaml:"hash"`
	Username string `yaml:"username"`
	UserId   string `yaml:"userID"`
}

func (c Dex) DexDNSNames() string {
	u, _ := url.Parse(c.Url)
	return u.Host
}
