package pki

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

// format for NotBefore and NotAfter fields to make output similar to openssl
var ValidityFormat = "Jan _2 15:04:05 2006 MST"

type Certificates []Certificate

// returns certificate that matches subject CN match regex (Subject.CommonName), if the certificate cannot be found,
// second returned value will be false
func (cs Certificates) GetBySubjectCommonNamePattern(subjectCNMatch string) (cert Certificate, ok bool) {

	for _, c := range cs {
		if match, _ := regexp.MatchString(subjectCNMatch, c.Subject.CommonName); match {
			return c, true
		}
	}
	return
}

type Certificate struct {
	Issuer      DN
	NotBefore   time.Time
	NotAfter    time.Time
	Subject     DN
	DNSNames    []string
	IPAddresses []net.IP
}

func (c Certificate) IsExpired() bool {
	return time.Now().After(c.NotAfter)
}

func (c Certificate) ContainsDNSName(name string) bool {

	for _, d := range c.DNSNames {
		if d == name {
			return true
		}
	}
	return false
}

func (c Certificate) ContainsIPAddress(ip net.IP) bool {

	for _, i := range c.IPAddresses {
		if i.Equal(ip) {
			return true
		}
	}
	return false
}

func (c Certificate) String() string {

	notBefore := c.NotBefore.Format(ValidityFormat)
	notAfter := c.NotAfter.Format(ValidityFormat)
	dnsNames := strings.Join(c.DNSNames, ", ")

	var ips []string
	for _, ip := range c.IPAddresses {
		ips = append(ips, fmt.Sprintf("%s", ip))
	}
	ipAddresses := strings.Join(ips, ", ")

	return fmt.Sprintf(
		"Issuer: %s\nValidity\n    Not Before: %s\n    Not After : %s\nSubject: %s\nDNS Names: %s\nIP Addresses: %s",
		c.Issuer, notBefore, notAfter, c.Subject, dnsNames, ipAddresses)
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
func CertificatesFromBytes(data []byte) (Certificates, error) {

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
				DNSNames:    c.DNSNames,
				IPAddresses: c.IPAddresses,
			},
		)
	}
	return certificates, nil
}
