package tlsutil

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"regexp"
	"time"
)

var (
	Duration365d = time.Hour * 24 * 365
)

type CACertConfig struct {
	CommonName   string
	Organization string
	Duration     time.Duration
}

type ServerCertConfig struct {
	CommonName  string
	DNSNames    []string
	IPAddresses []string
	Duration    time.Duration
}

type ClientCertConfig struct {
	CommonName   string
	Organization []string
	DNSNames     []string
	IPAddresses  []string
	Duration     time.Duration
}

func NewSelfSignedCACertificate(cfg CACertConfig, key *rsa.PrivateKey) (*x509.Certificate, error) {
	if cfg.Duration <= 0 {
		return nil, errors.New("Self-signed CA cert duration must not be negative or zero.")
	}

	tmpl := x509.Certificate{
		SerialNumber: new(big.Int).SetInt64(0),
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: []string{cfg.Organization},
		},
		NotBefore:             time.Now().UTC(),
		NotAfter:              time.Now().Add(cfg.Duration).UTC(),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA: true,
	}

	certDERBytes, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, key.Public(), key)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(certDERBytes)
}

func NewSignedServerCertificate(cfg ServerCertConfig, key *rsa.PrivateKey, caCert *x509.Certificate, caKey *rsa.PrivateKey) (*x509.Certificate, error) {
	ips := make([]net.IP, len(cfg.IPAddresses))
	for i, ipStr := range cfg.IPAddresses {
		ips[i] = net.ParseIP(ipStr)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}

	if cfg.Duration <= 0 {
		return nil, errors.New("Signed server cert duration must not be negative or zero.")
	}

	certTmpl := x509.Certificate{
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: caCert.Subject.Organization,
		},
		DNSNames:     cfg.DNSNames,
		IPAddresses:  ips,
		SerialNumber: serial,
		NotBefore:    caCert.NotBefore,
		NotAfter:     time.Now().Add(cfg.Duration).UTC(),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	certDERBytes, err := x509.CreateCertificate(rand.Reader, &certTmpl, caCert, key.Public(), caKey)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(certDERBytes)
}

func NewSignedClientCertificate(cfg ClientCertConfig, key *rsa.PrivateKey, caCert *x509.Certificate, caKey *rsa.PrivateKey) (*x509.Certificate, error) {
	ips := make([]net.IP, len(cfg.IPAddresses))
	for i, ipStr := range cfg.IPAddresses {
		ips[i] = net.ParseIP(ipStr)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}

	if cfg.Duration <= 0 {
		return nil, errors.New("Signed client cert duration must not be negative or zero.")
	}

	certTmpl := x509.Certificate{
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: append(caCert.Subject.Organization, cfg.Organization...),
		},
		DNSNames:     cfg.DNSNames,
		IPAddresses:  ips,
		SerialNumber: serial,
		NotBefore:    caCert.NotBefore,
		NotAfter:     time.Now().Add(cfg.Duration).UTC(),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	certDERBytes, err := x509.CreateCertificate(rand.Reader, &certTmpl, caCert, key.Public(), caKey)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(certDERBytes)
}

func NewSignedKIAMCertificate(cfg ClientCertConfig, key *rsa.PrivateKey, caCert *x509.Certificate, caKey *rsa.PrivateKey) (*x509.Certificate, error) {
	ips := make([]net.IP, len(cfg.IPAddresses))
	for i, ipStr := range cfg.IPAddresses {
		ips[i] = net.ParseIP(ipStr)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}

	if cfg.Duration <= 0 {
		return nil, errors.New("Signed client cert duration must not be negative or zero.")
	}

	certTmpl := x509.Certificate{
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: append(caCert.Subject.Organization, cfg.Organization...),
		},
		DNSNames:     cfg.DNSNames,
		IPAddresses:  ips,
		SerialNumber: serial,
		NotBefore:    caCert.NotBefore,
		NotAfter:     time.Now().Add(cfg.Duration).UTC(),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}
	certDERBytes, err := x509.CreateCertificate(rand.Reader, &certTmpl, caCert, key.Public(), caKey)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(certDERBytes)
}

func CheckAllCertsValid(filename string, contents []byte) error {
	var rest = contents
	for len(rest) > 0 {
		var cert *x509.Certificate
		var err error
		cert, rest, err = decodeCert(rest)
		if err != nil {
			return fmt.Errorf("failed to decode certificate: %v", err)
		}
		if cert == nil {
			continue
		}
		if certificateExpired(*cert) {
			info := fmt.Sprintf("Subject: %+v\nIssuer: %+v\nValid From: %s\nExpires: %s",
				cert.Subject,
				cert.Issuer,
				cert.NotBefore.String(),
				cert.NotAfter.String(),
			)
			return fmt.Errorf("The following certificate in file %s has expired:-\n\n%s", filename, info)
		}
	}
	return nil
}

func certificateExpired(c x509.Certificate) bool {
	return time.Now().After(c.NotAfter)
}

func decodeCert(c []byte) (*x509.Certificate, []byte, error) {
	p, rest := pem.Decode(c)
	if p == nil {
		return nil, rest, fmt.Errorf("Could not decode pem")
	}
	// skip over other pem blocks until we find a certificate
	for len(rest) > 0 {
		if p.Type == "CERTIFICATE" {
			break
		} else {
			p, rest := pem.Decode(c)
			if p == nil {
				return nil, rest, fmt.Errorf("Could not decode pem")
			}
		}
	}
	if p.Type == "CERTIFICATE" {
		cert, err := x509.ParseCertificate(p.Bytes)
		if err != nil {
			return nil, rest, fmt.Errorf("Could not decode x509 cert from pem")
		}
		return cert, rest, nil
	}
	return nil, rest, nil
}

func CertificateContainsDNSName(cert []byte, subjectMatch string, name string) (bool, error) {
	var rest = cert
	for len(rest) != 0 {
		var cert *x509.Certificate
		var err error
		cert, rest, err = decodeCert(rest)
		if err != nil {
			return false, fmt.Errorf("failed to decode certificate: %v", err)
		}
		match, _ := regexp.MatchString(subjectMatch, cert.Subject.CommonName)
		if match {
			for _, d := range cert.DNSNames {
				if d == name {
					return true, nil
				}
			}
			return false, nil
		}
	}
	return false, fmt.Errorf("no certificates containing match string: %s", subjectMatch)
}

func CertificateContainsIPAddress(cert []byte, subjectMatch string, ip net.IP) (bool, error) {
	var rest = cert
	for len(rest) != 0 {
		var cert *x509.Certificate
		var err error
		cert, rest, err = decodeCert(rest)
		if err != nil {
			return false, fmt.Errorf("failed to decode certificate: %v", err)
		}
		match, _ := regexp.MatchString(subjectMatch, cert.Subject.CommonName)
		if match {
			for _, i := range cert.IPAddresses {
				if i.Equal(ip) {
					return true, nil
				}
			}
			return false, nil
		}
	}
	return false, fmt.Errorf("no certificates containing match string: %s", subjectMatch)
}
