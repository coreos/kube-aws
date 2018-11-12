package model

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"github.com/kubernetes-incubator/kube-aws/credential"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"github.com/kubernetes-incubator/kube-aws/pki"
	"testing"
)

func genAssets(t *testing.T) *credential.RawAssetsOnMemory {
	c, err := ClusterFromBytes([]byte(singleAzConfigYaml))
	if err != nil {
		t.Fatalf("failed generating config: %v", err)
	}

	caKey, caCert, err := pki.NewCA(c.TLSCADurationDays)
	if err != nil {
		t.Fatalf("failed generating tls ca: %v", err)
	}
	cfg, err := Compile(c, api.ClusterOptions{})
	r := NewCredentialGenerator(cfg)
	assets, err := r.GenerateAssetsOnMemory(caKey, caCert, true)
	if err != nil {
		t.Fatalf("failed generating assets: %v", err)
	}

	return assets
}

func TestTLSGeneration(t *testing.T) {
	assets := genAssets(t)

	pairs := []*struct {
		Name      string
		KeyBytes  []byte
		CertBytes []byte
		Key       *rsa.PrivateKey
		Cert      *x509.Certificate
	}{
		//CA MUST come first
		{
			Name:      "ca",
			KeyBytes:  assets.CAKey,
			CertBytes: assets.CACert,
		},
		{
			Name:      "apiserver",
			KeyBytes:  assets.APIServerKey,
			CertBytes: assets.APIServerCert,
		},
		{
			Name:      "kube-controller-manager",
			KeyBytes:  assets.KubeControllerManagerKey,
			CertBytes: assets.KubeControllerManagerCert,
		},
		{
			Name:      "kube-scheduler",
			KeyBytes:  assets.KubeSchedulerKey,
			CertBytes: assets.KubeSchedulerCert,
		},
		{
			Name:      "apiserver-aggregator",
			KeyBytes:  assets.APIServerAggregatorKey,
			CertBytes: assets.APIServerAggregatorCert,
		},
		{
			Name:      "admin",
			KeyBytes:  assets.AdminKey,
			CertBytes: assets.AdminCert,
		},
		{
			Name:      "worker",
			KeyBytes:  assets.WorkerKey,
			CertBytes: assets.WorkerCert,
		},
		{
			Name:      "etcd",
			KeyBytes:  assets.EtcdKey,
			CertBytes: assets.EtcdCert,
		},
	}

	var err error
	for _, pair := range pairs {

		if keyBlock, _ := pem.Decode(pair.KeyBytes); keyBlock == nil {
			t.Errorf("Failed decoding pem block from %s", pair.Name)
		} else {
			pair.Key, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
			if err != nil {
				t.Errorf("Failed to parse key %s : %v", pair.Name, err)
			}
		}

		if certBlock, _ := pem.Decode(pair.CertBytes); certBlock == nil {
			t.Errorf("Failed decoding pem block from %s", pair.Name)
		} else {
			pair.Cert, err = x509.ParseCertificate(certBlock.Bytes)
			if err != nil {
				t.Errorf("Failed to parse cert %s: %v", pair.Name, err)
			}
		}
	}

	t.Log("Assets assets parsed successfully")

	if t.Failed() {
		t.Fatalf("Assets key pairs not parsed, cannot verify signatures")
	}

	caCert := pairs[0].Cert
	for _, pair := range pairs[1:] {
		if err := pair.Cert.CheckSignatureFrom(caCert); err != nil {
			t.Errorf(
				"Could not verify ca certificate signature %s : %v",
				pair.Name,
				err)
		}
	}
}
