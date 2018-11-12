package model

import (
	"io/ioutil"
	"os"
	"testing"

	"fmt"

	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/coreos/coreos-cloudinit/config/validate"
	"github.com/kubernetes-incubator/kube-aws/builtin"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"github.com/kubernetes-incubator/kube-aws/test/helper"
	"github.com/stretchr/testify/assert"
	"path/filepath"
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
	var cfg *Stack

	for _, cloudTemplate := range []struct {
		Name     string
		Template []byte
	}{
		{
			Name:     "CloudConfigController",
			Template: builtin.Bytes("userdata/cloud-config-controller"),
		},
	} {
		tmpfile, _ := ioutil.TempFile("", "ud")
		tmpfile.Write(cloudTemplate.Template)
		tmpfile.Close()
		defer os.Remove(tmpfile.Name())

		fname := tmpfile.Name()

		if fname == "" {
			t.Errorf("[bug] expected file name: %s", fname)
			t.FailNow()
		}

		pwd, err := os.Getwd()
		if err != nil {
			t.Errorf("%v", err)
			t.FailNow()
		}
		helper.WithTempDir(func(dir string) {
			opts := api.StackTemplateOptions{
				AssetsDir:             dir,
				ControllerTmplFile:    fname,
				StackTemplateTmplFile: filepath.Join(pwd, "../../builtin/files/stack-templates/control-plane.json.tmpl"),
			}
			cfg, err = yamlToStackForTesting(singleAzConfigYaml, opts)
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}

		udata := cfg.UserData["Controller"]
		content, err := udata.Parts[api.USERDATA_S3].Template()
		if !assert.NoError(t, err, "Can't render template %s", cloudTemplate.Name) {
			continue
		}

		report, err := validate.Validate([]byte(content))
		if !assert.NoError(t, err, "cloud-config %s could not be parsed", cloudTemplate.Name) {
			for _, entry := range report.Entries() {
				t.Errorf("%s: %+v", cloudTemplate.Name, entry)
			}
			continue
		}
	}
}
