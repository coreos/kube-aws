package credential

import (
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"github.com/kubernetes-incubator/kube-aws/pki"
	"io/ioutil"
	"os"
)

type ProtectedPKI struct {
	Encryptor
	*pki.PKI
}

func NewProtectedPKI(enc Encryptor) *ProtectedPKI {
	return &ProtectedPKI{
		Encryptor: enc,
		PKI:       pki.NewPKI(),
	}
}

func (pki *ProtectedPKI) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (pki *ProtectedPKI) write(path string, data []byte) error {
	return ioutil.WriteFile(path, data, 0644)
}

func (pki *ProtectedPKI) CreateKeyaPair(spec api.KeyPairSpec) error {
	keypair, err := pki.GenerateKeyPair(spec)
	if err != nil {
		return err
	}

	keypath := spec.KeyPath()
	keypem := keypair.KeyInPEM()
	if _, err := CreateEncryptedFile(keypath, keypem, pki); err != nil {
		return err
	}

	crtpath := spec.CertPath()
	crtpem := keypair.CertInPEM()
	if err := pki.write(crtpath, crtpem); err != nil {
		return err
	}

	return nil
}

func (pki *ProtectedPKI) EnsureKeyPairsCreated(specs []api.KeyPairSpec) error {
	for _, spec := range specs {
		keypath := spec.KeyPath()
		shapath := spec.KeyPath() + ".fingerprint"
		encpath := spec.EncryptedKeyPath()
		crtpath := spec.CertPath()
		if !pki.fileExists(keypath) && !pki.fileExists(encpath) && !pki.fileExists(shapath) && !pki.fileExists(crtpath) {
			if err := pki.CreateKeyaPair(spec); err != nil {
				return err
			}
		}
	}
	return nil
}
