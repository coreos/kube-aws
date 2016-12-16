package config

import (
	"bytes"
	"testing"
	"text/template"

	"fmt"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/coreos/coreos-cloudinit/config/validate"
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
	caKey, caCert, err := cluster.NewTLSCA()
	if err != nil {
		t.Fatalf("failed generating tls ca: %v", err)
	}
	assets, err := cluster.NewTLSAssets(caKey, caCert)
	if err != nil {
		t.Fatalf("Error generating default assets: %v", err)
	}

	cfg, err := cluster.Config()
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	encryptedAssets, err := assets.Encrypt(cfg.KMSKeyARN, &dummyEncryptService{})
	if err != nil {
		t.Fatalf("failed to compress TLS assets: %v", err)
	}

	compactAssets, err := encryptedAssets.Compact()
	if err != nil {
		t.Fatalf("failed to compress TLS assets: %v", err)
	}

	cfg.TLSConfig = compactAssets

	for _, cloudTemplate := range []struct {
		Name     string
		Template []byte
	}{
		{
			Name:     "CloudConfigWorker",
			Template: CloudConfigWorker,
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
