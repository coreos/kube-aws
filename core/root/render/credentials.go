package render

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"github.com/coreos/kube-aws/core/controlplane/config"
	"github.com/coreos/kube-aws/core/root/defaults"
	"github.com/coreos/kube-aws/tlsutil"
	"io/ioutil"
	"os"
)

type CredentialsOptions struct {
	GenerateCA bool
	CaKeyPath  string
	CaCertPath string
}

type CredentialsRenderer interface {
	RenderFiles(CredentialsOptions) error
}

type credentialsRendererImpl struct {
	c *config.Cluster
}

func NewCredentialsRenderer(c *config.Cluster) CredentialsRenderer {
	return credentialsRendererImpl{
		c: c,
	}
}

func (r credentialsRendererImpl) RenderFiles(renderCredentialsOpts CredentialsOptions) error {
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
	assets, err := cluster.NewTLSAssets(caKey, caCert)
	if err != nil {
		return fmt.Errorf("Error generating default assets: %v", err)
	}
	if err := assets.WriteToDir(dir, renderCredentialsOpts.GenerateCA); err != nil {
		return fmt.Errorf("Error create assets: %v", err)
	}

	fmt.Printf("-> Generating auth token file\n")
	authToken := cluster.NewAuthTokens()
	if err := authToken.WriteToDir(dir); err != nil {
		return fmt.Errorf("Error create auth token file: %v", err)
	}

	return nil
}
