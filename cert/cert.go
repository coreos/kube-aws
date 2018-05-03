package cert

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"
)

// format for NotBefore and NotAfter fields to make output similar to openssl
var ValidityFormat = "Jan _2 15:04:05 2006 MST"

type Certificate struct {
	Issuer    DN
	NotBefore time.Time
	NotAfter  time.Time
	Subject   DN
	DNSNames  []string
}

func (c Certificate) String() string {

	notBefore := c.NotBefore.Format(ValidityFormat)
	notAfter := c.NotAfter.Format(ValidityFormat)
	dnsNames := strings.Join(c.DNSNames, ", ")

	return fmt.Sprintf("Issuer: %s\nValidity\n    Not Before: %s\n    Not After : %s\nSubject: %s\nDNS Names: %s",
		c.Issuer, notBefore, notAfter, c.Subject, dnsNames)
}

type DN struct {
	Organization []string
	CommonName   string
}

func (dn DN) String() string {

	var fields []string
	if len(dn.Organization) != 0 {
		fields = append(fields, fmt.Sprintf("O=%s", strings.Join(dn.Organization, ", ")))
	}
	if dn.CommonName != "" {
		fields = append(fields, fmt.Sprintf("CN=%s", dn.CommonName))
	}
	return strings.Join(fields, " ")
}

func ParseCertificates(certs []byte) ([]Certificate, error) {

	blocks, err := decodeCertificates(certs)
	if err != nil {
		return nil, err
	}

	cs, err := x509.ParseCertificates(blocks)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %v", err)
	}

	var certificates []Certificate
	for _, c := range cs {
		certificates = append(
			certificates,
			Certificate{
				Issuer: DN{
					Organization: c.Issuer.Organization,
					CommonName:   c.Issuer.CommonName,
				},
				NotAfter:  c.NotAfter,
				NotBefore: c.NotBefore,
				Subject: DN{
					Organization: c.Subject.Organization,
					CommonName:   c.Subject.CommonName,
				},
				DNSNames: c.DNSNames,
			},
		)
	}
	return certificates, nil
}

func IsCertificate(data []byte) bool {
	block, _ := pem.Decode(data)
	return block != nil && block.Type == "CERTIFICATE"
}

func decodeCertificates(rawCerts []byte) ([]byte, error) {

	var block *pem.Block
	var decodedCerts []byte
	for {
		block, rawCerts = pem.Decode(rawCerts)
		if block == nil {
			return nil, errors.New("failed to parse certificate PEM")
		}
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("failed to parse %s, only CERTIFICATE can be parsed", block.Type)
		}
		decodedCerts = append(decodedCerts, block.Bytes...)
		if len(rawCerts) == 0 {
			break
		}
	}
	return decodedCerts, nil
}
