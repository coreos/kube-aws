package pki

import (
	"crypto/rand"
	"crypto/rsa"
)

const (
	RSAKeySize = 2048
)

func NewPrivateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, RSAKeySize)
}
