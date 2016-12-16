package config

import (
	"testing"

	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"github.com/coreos/kube-aws/test/helper"
	"os"
	"path/filepath"
	"reflect"
)

func genTLSAssets(t *testing.T) *RawTLSAssets {
	cluster, err := ClusterFromBytes([]byte(singleAzConfigYaml))
	if err != nil {
		t.Fatalf("failed generating config: %v", err)
	}

	caKey, caCert, err := cluster.NewTLSCA()
	if err != nil {
		t.Fatalf("failed generating tls ca: %v", err)
	}
	assets, err := cluster.NewTLSAssets(caKey, caCert)
	if err != nil {
		t.Fatalf("failed generating tls: %v", err)
	}

	return assets
}

func TestTLSGeneration(t *testing.T) {
	assets := genTLSAssets(t)

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

	t.Log("TLS assets parsed successfully")

	if t.Failed() {
		t.Fatalf("TLS key pairs not parsed, cannot verify signatures")
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

func TestReadOrCreateCompactTLSAssets(t *testing.T) {
	helper.WithDummyCredentials(func(dir string) {
		kmsConfig := KMSConfig{
			KMSKeyARN:      "keyarn",
			Region:         "us-west-1",
			EncryptService: &dummyEncryptService{},
		}

		// See https://github.com/coreos/kube-aws/issues/107
		t.Run("CachedToPreventUnnecessaryNodeReplacement", func(t *testing.T) {
			created, err := ReadOrCreateCompactTLSAssets(dir, kmsConfig)

			if err != nil {
				t.Errorf("failed to read or update compact tls assets in %s : %v", dir, err)
			}

			// This depends on TestDummyEncryptService which ensures dummy encrypt service to produce different ciphertext for each encryption
			// created == read means that encrypted assets were loaded from cached files named *.pem.enc, instead of re-encryptiong raw tls assets named *.pem files
			// TODO Use some kind of mocking framework for tests like this
			read, err := ReadOrCreateCompactTLSAssets(dir, kmsConfig)

			if err != nil {
				t.Errorf("failed to read or update compact tls assets in %s : %v", dir, err)
			}

			if !reflect.DeepEqual(created, read) {
				t.Errorf(`failed to cache encrypted tls assets.
	encrypted tls assets must not change after their first creation but they did change:
	created = %v
	read = %v`, created, read)
			}
		})

		t.Run("RemoveOneOrMoreCacheFilesToRegenerateAll", func(t *testing.T) {
			original, err := ReadOrCreateCompactTLSAssets(dir, kmsConfig)

			if err != nil {
				t.Errorf("failed to read the original encrypted tls assets : %v", err)
			}

			if err := os.Remove(filepath.Join(dir, "ca.pem.enc")); err != nil {
				t.Errorf("failed to remove ca.pem.enc for test setup : %v", err)
				t.FailNow()
			}

			regenerated, err := ReadOrCreateCompactTLSAssets(dir, kmsConfig)

			if err != nil {
				t.Errorf("failed to read the regenerated encrypted tls assets : %v", err)
			}

			if original.AdminCert == regenerated.AdminCert {
				t.Errorf("AdminCert must change but it didn't : original = %v, regenrated = %v ", original.AdminCert, regenerated.AdminCert)
			}

			if original.AdminKey == regenerated.AdminKey {
				t.Errorf("AdminKey must change but it didn't : original = %v, regenrated = %v ", original.AdminKey, regenerated.AdminKey)
			}

			if original.CACert == regenerated.CACert {
				t.Errorf("CACert must change but it didn't : original = %v, regenrated = %v ", original.CACert, regenerated.CACert)
			}

			if original.CACert == regenerated.CACert {
				t.Errorf("CACert must change but it didn't : original = %v, regenrated = %v ", original.CACert, regenerated.CACert)
			}

			if original.WorkerCert == regenerated.WorkerCert {
				t.Errorf("WorkerCert must change but it didn't : original = %v, regenrated = %v ", original.WorkerCert, regenerated.WorkerCert)
			}

			if original.WorkerCert == regenerated.WorkerCert {
				t.Errorf("WorkerCert must change but it didn't : original = %v, regenrated = %v ", original.WorkerCert, regenerated.WorkerCert)
			}

			if original.APIServerCert == regenerated.APIServerCert {
				t.Errorf("APIServerCert must change but it didn't : original = %v, regenrated = %v ", original.APIServerCert, regenerated.APIServerCert)
			}

			if original.APIServerCert == regenerated.APIServerCert {
				t.Errorf("APIServerCert must change but it didn't : original = %v, regenrated = %v ", original.APIServerCert, regenerated.APIServerCert)
			}

			if original.EtcdClientCert == regenerated.EtcdClientCert {
				t.Errorf("EtcdClientCert must change but it didn't : original = %v, regenrated = %v ", original.EtcdClientCert, regenerated.EtcdClientCert)
			}

			if original.EtcdClientCert == regenerated.EtcdClientCert {
				t.Errorf("EtcdClientCert must change but it didn't : original = %v, regenrated = %v ", original.EtcdClientCert, regenerated.EtcdClientCert)
			}

			if original.EtcdCert == regenerated.EtcdCert {
				t.Errorf("EtcdCert must change but it didn't : original = %v, regenrated = %v ", original.EtcdCert, regenerated.EtcdCert)
			}

			if original.EtcdCert == regenerated.EtcdCert {
				t.Errorf("EtcdCert must change but it didn't : original = %v, regenrated = %v ", original.EtcdCert, regenerated.EtcdCert)
			}

			if reflect.DeepEqual(original, regenerated) {
				t.Errorf(`unexpecteed data contained in (possibly) regenerated encrypted tls assets.
	encrypted tls assets must change after regeneration but they didn't:
	original = %v
	regenerated = %v`, original, regenerated)
			}
		})
	})
}
