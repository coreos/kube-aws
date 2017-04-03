package config

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/kms"
)

type EncryptService interface {
	Encrypt(*kms.EncryptInput) (*kms.EncryptOutput, error)
}

type bytesEncryptionService struct {
	kmsKeyARN string
	kmsSvc    EncryptService
}

func (s bytesEncryptionService) Encrypt(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}

	encryptInput := kms.EncryptInput{
		KeyId:     aws.String(s.kmsKeyARN),
		Plaintext: data,
	}
	encryptOutput, err := s.kmsSvc.Encrypt(&encryptInput)
	if err != nil {
		return []byte{}, err
	}
	return encryptOutput.CiphertextBlob, nil
}
