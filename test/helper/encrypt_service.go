package helper

import "github.com/aws/aws-sdk-go/service/kms"

type DummyEncryptService struct{}

func (d DummyEncryptService) Encrypt(input *kms.EncryptInput) (*kms.EncryptOutput, error) {
	output := kms.EncryptOutput{
		CiphertextBlob: input.Plaintext,
	}
	return &output, nil
}
