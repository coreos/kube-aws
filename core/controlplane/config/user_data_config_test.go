package config

import (
	"bytes"
	"testing"
	"text/template"

	"fmt"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/coreos/coreos-cloudinit/config/validate"
	"github.com/kubernetes-incubator/kube-aws/test/helper"
)

var numEncryption int

type dummyEncryptService struct{}

func (d *dummyEncryptService) Encrypt(input *kms.EncryptInput) (*kms.EncryptOutput, error) {
	output := kms.EncryptOutput{
		CiphertextBlob: []byte(fmt.Sprintf("%s%d", string(input.Plaintext), numEncryption)),
	}
	numEncryption += 1
	return &output, nil
}

func TestDummyEncryptService(t *testing.T) {
	encService := &dummyEncryptService{}
	plaintext := []byte("mysecretinformation")

	first, err := encService.Encrypt(&kms.EncryptInput{
		Plaintext: plaintext,
	})

	if err != nil {
		t.Errorf("failed to encrypt data %plaintext : %v", plaintext, err)
	}

	second, err := encService.Encrypt(&kms.EncryptInput{
		Plaintext: plaintext,
	})

	if err != nil {
		t.Errorf("failed to encrypt data %plaintext : %v", plaintext, err)
	}

	if first == second {
		t.Errorf("dummy encrypt service should produce different ciphertext for each encryption but it didnt: first = %v, second = %v", first, second)
	}
}

func TestCloudConfigTemplating(t *testing.T) {
	cluster, err := ClusterFromBytes([]byte(singleAzConfigYaml))
	if err != nil {
		t.Fatalf("Unable to load cluster config: %v", err)
	}

	cfg, err := cluster.Config()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// TLS assets
	caKey, caCert, err := cluster.NewTLSCA()
	if err != nil {
		t.Fatalf("failed generating tls ca: %v", err)
	}
	opts := CredentialsOptions{
		GenerateCA: true,
	}

	var compactAssets *CompactTLSAssets

	cachedEncryptor := CachedEncryptor{
		bytesEncryptionService: bytesEncryptionService{kmsKeyARN: cfg.KMSKeyARN, kmsSvc: &dummyEncryptService{}},
	}

	helper.WithTempDir(func(dir string) {
		_, err = cluster.NewTLSAssetsOnDisk(dir, opts, caKey, caCert)
		if err != nil {
			t.Fatalf("Error generating default assets: %v", err)
		}

		encryptedAssets, err := ReadOrEncryptTLSAssets(dir, cachedEncryptor)
		if err != nil {
			t.Fatalf("failed to compress TLS assets: %v", err)
		}

		compactAssets, err = encryptedAssets.Compact()
		if err != nil {
			t.Fatalf("failed to compress TLS assets: %v", err)
		}
	})

	if compactAssets == nil {
		t.Fatal("compactAssets is unexpectedly nil")
		t.FailNow()
	}

	cfg.TLSConfig = compactAssets

	var compactAuthTokens *CompactAuthTokens

	// Auth tokens
	helper.WithTempDir(func(dir string) {
		if _, err := CreateRawAuthTokens(false, dir); err != nil {
			t.Fatalf("failed to create auth token file: %v", err)
		}

		encryptedAuthTokens, err := ReadOrEncryptAuthTokens(dir, cachedEncryptor)
		if err != nil {
			t.Fatalf("failed to compress auth token file: %v", err)
		}

		compactAuthTokens, err = encryptedAuthTokens.Compact()
		if err != nil {
			t.Fatalf("failed to compress auth token file: %v", err)
		}
	})

	if compactAuthTokens == nil {
		t.Fatal("compactAuthTokens is unexpectedly nil")
		t.FailNow()
	}

	cfg.AuthTokensConfig = compactAuthTokens

	for _, cloudTemplate := range []struct {
		Name     string
		Template []byte
	}{
		{
			Name:     "CloudConfigEtcd",
			Template: CloudConfigEtcd,
		},
		{
			Name:     "CloudConfigController",
			Template: CloudConfigController,
		},
	} {
		tmpl, err := template.New(cloudTemplate.Name).Parse(string(cloudTemplate.Template))
		if err != nil {
			t.Errorf("Error loading template %s : %v", cloudTemplate.Name, err)
			continue
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, cfg); err != nil {
			t.Errorf("Error excuting template %s : %v", cloudTemplate.Name, err)
			continue
		}

		report, err := validate.Validate(buf.Bytes())

		if err != nil {
			t.Errorf("cloud-config %s could not be parsed: %v", cloudTemplate.Name, err)
			continue
		}

		for _, entry := range report.Entries() {
			t.Errorf("%s: %+v", cloudTemplate.Name, entry)
		}
	}
}
