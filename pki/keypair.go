package pki

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
)

func KeyPairFromPEMs(id string, certpem []byte, keypem []byte) (*KeyPair, error) {
	var cert *x509.Certificate
	var key *rsa.PrivateKey
	var err error
	if cert, err = DecodeCertificatePEM(certpem); err != nil {
		return nil, fmt.Errorf("failed to decode certificate pem: %v", err)
	}
	if key, err = DecodePrivateKeyPEM(keypem); err != nil {
		return nil, fmt.Errorf("failed to decode private key pem: %v", err)
	}
	kp := KeyPair{
		Key:     key,
		Cert:    cert,
		id:      id,
		keyPem:  keypem,
		certPem: certpem,
	}
	return &kp, nil
}

func (keypair *KeyPair) KeyInPEM() []byte {
	if keypair.keyPem == nil {
		keypair.keyPem = EncodePrivateKeyPEM(keypair.Key)
	}
	return keypair.keyPem
}

func (keypair *KeyPair) CertInPEM() []byte {
	if keypair.certPem == nil {
		keypair.certPem = EncodeCertificatePEM(keypair.Cert)
	}
	return keypair.certPem
}
