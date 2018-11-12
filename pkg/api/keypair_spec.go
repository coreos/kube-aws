package api

import (
	"fmt"
	"path/filepath"
	"time"
)

type KeyPairSpec struct {
	Name         string        `yaml:"name"`
	CommonName   string        `yaml:"commonName"`
	Organization string        `yaml:"organization"`
	Duration     time.Duration `yaml:"duration"`
	DNSNames     []string      `yaml:"dnsNames"`
	IPAddresses  []string      `yaml:"ipAddresses"`
	Usages       []string      `yaml:"usages"`
	// Signer is the name of the keypair for the private key used to sign the cert
	Signer string `yaml:"signer"`
}

func (spec KeyPairSpec) EncryptedKeyPath() string {
	return fmt.Sprintf("%s.enc", spec.KeyPath())
}

func (spec KeyPairSpec) KeyPath() string {
	return filepath.Join("credentials", fmt.Sprintf("%s-key.pem", spec.Name))
}

func (spec KeyPairSpec) CertPath() string {
	return filepath.Join("credentials", fmt.Sprintf("%s.pem", spec.Name))
}
