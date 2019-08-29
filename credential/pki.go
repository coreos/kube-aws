package credential

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"github.com/kubernetes-incubator/kube-aws/pki"
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

func (ppki *ProtectedPKI) CreateKeyaPair(spec api.KeyPairSpec) error {
	var signer *pki.KeyPair
	if spec.Signer != "" {
		signerCert, err := ioutil.ReadFile(spec.SignerCertPath())
		if err != nil {
			return fmt.Errorf("failed to read signer certificate %s for creating %s: %v", spec.SignerCertPath(), spec.Name, err)
		}
		signerKey, err := ioutil.ReadFile(spec.SignerKeyPath())
		if err != nil {
			return fmt.Errorf("failed to read signer key %s for creating %s: %v", spec.SignerKeyPath(), spec.Name, err)
		}
		signer, err = pki.KeyPairFromPEMs(spec.Signer, signerCert, signerKey)
	}
	keypair, err := ppki.GenerateKeyPair(spec, signer)
	if err != nil {
		return err
	}

	keypath := spec.KeyPath()
	keypem := keypair.KeyInPEM()
	logger.Infof("Writing key pem file %s", keypath)
	if err := ioutil.WriteFile(keypath, keypem, 0644); err != nil {
		return err
	}

	crtpath := spec.CertPath()
	crtpem := keypair.CertInPEM()
	logger.Infof("Writing certificate pem file %s", crtpath)
	if err := ioutil.WriteFile(crtpath, crtpem, 0644); err != nil {
		return err
	}

	return nil
}

func (ppki *ProtectedPKI) EnsureKeyPairsCreated(specs []api.KeyPairSpec) error {
	for _, spec := range specs {
		keypath := spec.KeyPath()
		shapath := spec.KeyPath() + ".fingerprint"
		encpath := spec.EncryptedKeyPath()
		crtpath := spec.CertPath()
		if !fileExists(keypath) && !fileExists(encpath) && !fileExists(shapath) && !fileExists(crtpath) {
			if err := ppki.CreateKeyaPair(spec); err != nil {
				return err
			}
		}
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
