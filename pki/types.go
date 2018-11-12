package pki

import (
	"crypto/rsa"
	"crypto/x509"
)

// KeyPair is the TLS public certificate PEM file and its associated private key PEM file that is
// used by kube-aws and its plugins
type KeyPair struct {
	Key  *rsa.PrivateKey
	Cert *x509.Certificate

	id string

	keyPem  []byte
	certPem []byte
}
