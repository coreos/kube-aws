package pki

import (
	"crypto/rsa"
	"crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEncodePrivateKeyPEM(t *testing.T) {

	key := getPrivateKey(t)
	b := EncodePrivateKeyPEM(key)
	decodedKey, err := DecodePrivateKeyPEM(b)
	require.NoError(t, err)

	assert.Equal(t, key, decodedKey)
}

func TestEncodeCertificatePEM(t *testing.T) {

	cert := getSelfSignedCert(t, "test CN", "ABC organization")
	b := EncodeCertificatePEM(cert)
	decodedCert, err := DecodeCertificatePEM(b)
	require.NoError(t, err)

	assert.Equal(t, cert, decodedCert)
}

func TestEncodeCertificatesPEM(t *testing.T) {

	cert1 := EncodeCertificatePEM(getSelfSignedCert(t, "test CN", "abc organization"))
	cert2 := EncodeCertificatePEM(getSelfSignedCert(t, "test 2 CN", "xyz organization"))
	bundle := append(cert1[:], cert2[:]...)

	decodedBundle, err := DecodeCertificatesPEM(bundle)
	require.NoError(t, err)

	assert.Equal(t, 2, len(decodedBundle))
}

func TestEncodeCertificatesPEMBundleContainsPrivateKey(t *testing.T) {

	cert1 := EncodeCertificatePEM(getSelfSignedCert(t, "test CN", "abc organization"))
	key := EncodePrivateKeyPEM(getPrivateKey(t))
	bundle := append(cert1[:], key[:]...)

	decodedBundle, err := DecodeCertificatesPEM(bundle)
	require.NoError(t, err)

	assert.Equal(t, 1, len(decodedBundle))
}

func TestIsCertificatePEMIsFalseForPrivateKey(t *testing.T) {

	key := getPrivateKey(t)
	b := EncodePrivateKeyPEM(key)
	isCert := IsCertificatePEM(b)

	assert.False(t, isCert)
}

func TestIsCertficatePEMIsTrueForSelfSignedCert(t *testing.T) {

	cert := getSelfSignedCert(t, "test CN", "ABC organization")
	b := EncodeCertificatePEM(cert)
	isCert := IsCertificatePEM(b)

	assert.True(t, isCert)
}

// --- helper functions ---

func getPrivateKey(t *testing.T) *rsa.PrivateKey {

	key, err := NewPrivateKey()
	require.NoError(t, err)
	return key
}

func getSelfSignedCert(t *testing.T, commonName, organization string) *x509.Certificate {

	key := getPrivateKey(t)
	cfg := CACertConfig{Duration: Duration365d, CommonName: commonName, Organization: organization}

	cert, err := NewSelfSignedCACertificate(cfg, key)
	require.NoError(t, err)

	return cert
}
