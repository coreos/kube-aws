package tlsutil

import (
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

// converts raw certificate bytes to certificate, if the supplied data is cert bundle (or chain)
// all the certificates will be returned
func ToCertificates(data []byte) ([]Certificate, error) {

	cs, err := DecodeCertificatesPEM(data)
	if err != nil {
		return nil, err
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
