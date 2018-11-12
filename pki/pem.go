package pki

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"errors"
	"golang.org/x/crypto/ssh/terminal"
)

const certificateType = "CERTIFICATE"

func EncodePrivateKeyPEM(key *rsa.PrivateKey) []byte {
	block := pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	return pem.EncodeToMemory(&block)
}

func promptPassphrase(prompt string) ([]byte, error) {
	fmt.Print(prompt)
	passphrase, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	fmt.Print("\n")
	return passphrase, err
}

func DecodePrivateKeyPEM(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	var blockBytes []byte
	if x509.IsEncryptedPEMBlock(block) {
		var passphrase []byte
		var err error
		passphrase_env := os.Getenv("KUBE_AWS_CA_KEY_PASSPHRASE")
		if passphrase_env != "" {
			passphrase = []byte(passphrase_env)
		} else {
			passphrase, err = promptPassphrase("CA Key passphrase: ")
			if err != nil {
				return nil, err
			}
		}
		blockBytes, err = x509.DecryptPEMBlock(block, passphrase)
		if err != nil {
			return nil, err
		}
	} else {
		blockBytes = block.Bytes
	}
	return x509.ParsePKCS1PrivateKey(blockBytes)
}

func EncodeCertificatePEM(cert *x509.Certificate) []byte {
	block := pem.Block{
		Type:  certificateType,
		Bytes: cert.Raw,
	}
	return pem.EncodeToMemory(&block)
}

func DecodeCertificatePEM(data []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(data)
	return x509.ParseCertificate(block.Bytes)
}

func DecodeCertificatesPEM(data []byte) ([]*x509.Certificate, error) {
	var block *pem.Block
	var decodedCerts []byte
	for {
		block, data = pem.Decode(data)
		if block == nil {
			return nil, errors.New("failed to parse certificate PEM")
		}
		// append only certificates
		if block.Type == certificateType {
			decodedCerts = append(decodedCerts, block.Bytes...)
		}
		if len(data) == 0 {
			break
		}
	}
	return x509.ParseCertificates(decodedCerts)
}

func IsCertificatePEM(data []byte) bool {
	block, _ := pem.Decode(data)
	return block != nil && block.Type == certificateType
}
