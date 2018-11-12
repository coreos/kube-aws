package pki

import (
	"crypto/rsa"
	"crypto/x509"
	"time"
)

func NewCA(caDurationDays int) (*rsa.PrivateKey, *x509.Certificate, error) {
	caKey, err := NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}

	// Convert from days to time.Duration
	caDuration := time.Duration(caDurationDays) * 24 * time.Hour

	caConfig := CACertConfig{
		CommonName:   "kube-ca",
		Organization: "kube-aws",
		Duration:     caDuration,
	}
	caCert, err := NewSelfSignedCACertificate(caConfig, caKey)
	if err != nil {
		return nil, nil, err
	}

	return caKey, caCert, nil
}
