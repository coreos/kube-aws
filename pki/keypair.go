package pki

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
