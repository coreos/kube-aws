package credential

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/kms"
)

func (s KMSEncryptor) EncryptedBytes(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}

	encryptInput := kms.EncryptInput{
		KeyId:     aws.String(s.KmsKeyARN),
		Plaintext: data,
	}
	encryptOutput, err := s.KmsSvc.Encrypt(&encryptInput)
	if err != nil {
		return []byte{}, err
	}
	return encryptOutput.CiphertextBlob, nil
}
