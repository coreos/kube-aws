package render

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	"github.com/kubernetes-incubator/kube-aws/core/root/defaults"
	"github.com/kubernetes-incubator/kube-aws/tlsutil"
	"io/ioutil"
	"os"
)

type CredentialsRenderer interface {
	RenderFiles(config.CredentialsOptions) error
}

type credentialsRendererImpl struct {
	c *config.Cluster
}

func NewCredentialsRenderer(c *config.Cluster) CredentialsRenderer {
	return credentialsRendererImpl{
		c: c,
	}
}

func (r credentialsRendererImpl) RenderFiles(renderCredentialsOpts config.CredentialsOptions) error {
	cluster := r.c
	fmt.Printf("Generating TLS credentials...\n")
	var caKey *rsa.PrivateKey
	var caCert *x509.Certificate
	if renderCredentialsOpts.GenerateCA {
		var err error
		caKey, caCert, err = cluster.NewTLSCA()
		if err != nil {
			return fmt.Errorf("failed generating cluster CA: %v", err)
		}
		fmt.Printf("-> Generating new TLS CA\n")
	} else {
		fmt.Printf("-> Parsing existing TLS CA\n")
		if caKeyBytes, err := ioutil.ReadFile(renderCredentialsOpts.CaKeyPath); err != nil {
			return fmt.Errorf("failed reading ca key file %s : %v", renderCredentialsOpts.CaKeyPath, err)
		} else {
			if caKey, err = tlsutil.DecodePrivateKeyPEM(caKeyBytes); err != nil {
				return fmt.Errorf("failed parsing ca key: %v", err)
			}
		}
		if caCertBytes, err := ioutil.ReadFile(renderCredentialsOpts.CaCertPath); err != nil {
			return fmt.Errorf("failed reading ca cert file %s : %v", renderCredentialsOpts.CaCertPath, err)
		} else {
			if caCert, err = tlsutil.DecodeCertificatePEM(caCertBytes); err != nil {
				return fmt.Errorf("failed parsing ca cert: %v", err)
			}
		}
	}

	dir := defaults.AssetsDir
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	fmt.Printf("-> Generating new TLS assets\n")
	_, err := cluster.NewTLSAssetsOnDisk(dir, renderCredentialsOpts, caKey, caCert)
	if err != nil {
		return err
	}

	fmt.Printf("-> Generating auth token file\n")
	_, err = config.NewAuthTokensOnDisk(dir)
	if err != nil {
		return err
	}

	return nil
}
