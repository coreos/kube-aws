package tlsutil

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
	"time"
)

func TestIsExpired(t *testing.T) {

	cert := Certificate{NotAfter: time.Now().AddDate(0, 0, -1)}
	assert.True(t, cert.IsExpired())
}

func TestIsNotExpired(t *testing.T) {

	cert := Certificate{NotAfter: time.Now().AddDate(0, 0, 1)}
	assert.False(t, cert.IsExpired())
}

func TestCertificateContainsDNSName(t *testing.T) {

	cert := Certificate{DNSNames: []string{"kube-aws.com", "test.com"}}
	assert.True(t, cert.ContainsDNSName("kube-aws.com"))
}

func TestCertificateDoesNOTContainDNSName(t *testing.T) {

	cert := Certificate{}
	assert.False(t, cert.ContainsDNSName("kube-aws.com"))
}

func TestCertifcateContainsIPAddress(t *testing.T) {

	localhost := net.IPv4(127, 0, 0, 1)
	cert := Certificate{IPAddresses: []net.IP{localhost}}
	assert.True(t, cert.ContainsIPAddress(localhost))
}

func TestCertifcateDoesNOTContainIPAddress(t *testing.T) {

	localhost := net.IPv4(127, 0, 0, 1)
	cert := Certificate{}
	assert.False(t, cert.ContainsIPAddress(localhost))
}

func TestToCertificates(t *testing.T) {

	cert1 := EncodeCertificatePEM(getSelfSignedCert(t, "test CN", "ABC organization"))
	cert2 := EncodeCertificatePEM(getSelfSignedCert(t, "test 2 CN", "XYZ organization"))
	bundle := append(cert1[:], cert2[:]...)
	certs, err := ToCertificates(bundle)
	require.NoError(t, err)

	require.Equal(t, 2, len(certs))
	assert.Equal(t, "test CN", certs[0].Issuer.CommonName)
	assert.Equal(t, "test CN", certs[0].Subject.CommonName)
	assert.Equal(t, "test 2 CN", certs[1].Issuer.CommonName)
	assert.Equal(t, "test 2 CN", certs[1].Subject.CommonName)

	require.Equal(t, 1, len(certs[0].Issuer.Organization))
	require.Equal(t, 1, len(certs[0].Subject.Organization))
	assert.Equal(t, "ABC organization", certs[0].Issuer.Organization[0])
	assert.Equal(t, "ABC organization", certs[0].Subject.Organization[0])
}

func TestGetCertificateExistsInBundle(t *testing.T) {

	cert1 := EncodeCertificatePEM(getSelfSignedCert(t, "one", ""))
	cert2 := EncodeCertificatePEM(getSelfSignedCert(t, "two", ""))
	bundle := append(cert1[:], cert2[:]...)
	certs, err := ToCertificates(bundle)
	require.NoError(t, err)

	_, ok := GetCertificate(certs, "two")
	assert.True(t, ok)
}

func TestGetCertificateMissingFromBundle(t *testing.T) {

	cert1 := EncodeCertificatePEM(getSelfSignedCert(t, "one", ""))
	cert2 := EncodeCertificatePEM(getSelfSignedCert(t, "two", ""))
	bundle := append(cert1[:], cert2[:]...)
	certs, err := ToCertificates(bundle)
	require.NoError(t, err)

	_, ok := GetCertificate(certs, "three")
	assert.False(t, ok)
}
