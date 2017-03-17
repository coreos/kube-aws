package config

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"

	"github.com/coreos/kube-aws/gzipcompressor"
)

// Contents of the CSV file holding auth tokens.
// See https://kubernetes.io/docs/admin/authentication/#static-token-file
type AuthTokens struct {
	Contents []byte
}

// Contents of the CSV file holding auth tokens.
type RawAuthTokens struct {
	AuthTokens
}

// Encrypted contents of the CSV file holding auth tokens.
type EncryptedAuthTokens struct {
	AuthTokens
}

// Encrypted -> gzip -> base64 encoded auth token file contents.
type CompactAuthTokens struct {
	Contents string
}

func (c *Cluster) NewAuthTokens() *RawAuthTokens {
	// Uses an empty file as the default auth token file
	return &RawAuthTokens{AuthTokens{
		Contents: make([]byte, 0),
	}}
}

func validateAuthTokens(authTokens []byte) error {
	if len(authTokens) > 0 {
		csvReader := csv.NewReader(bytes.NewReader(authTokens))

		records, err := csvReader.ReadAll()
		if err != nil {
			return err
		}

		for _, line := range records {
			columns := len(line)
			if columns < 3 {
				return fmt.Errorf("auth token record must have at least 3 columns, but has %d: '%v'", columns, line)
			}
		}
	}

	return nil
}

func ReadRawAuthTokens(dirname string) (*RawAuthTokens, error) {
	authTokensPath := filepath.Join(dirname, "tokens.csv")
	authTokens, err := ioutil.ReadFile(authTokensPath)
	if err != nil {
		return nil, err
	}

	if err = validateAuthTokens(authTokens); err != nil {
		return nil, err
	}

	return &RawAuthTokens{AuthTokens{
		Contents: authTokens,
	}}, nil
}

func ReadEncryptedAuthTokens(dirname string) (*EncryptedAuthTokens, error) {
	authTokensEncPath := filepath.Join(dirname, "tokens.csv.enc")
	authTokensEnc, err := ioutil.ReadFile(authTokensEncPath)

	if err != nil {
		return nil, err
	}

	return &EncryptedAuthTokens{AuthTokens{
		Contents: authTokensEnc,
	}}, nil
}

func (r *RawAuthTokens) WriteToDir(dirname string) error {
	authTokensPath := filepath.Join(dirname, "tokens.csv")

	if err := ioutil.WriteFile(authTokensPath, r.Contents, 0600); err != nil {
		return err
	}

	return nil
}

func (r *RawAuthTokens) Encrypt(kMSKeyARN string, kmsSvc EncryptService) (*EncryptedAuthTokens, error) {
	var err error
	encrypt := func(data []byte) []byte {
		if len(data) == 0 {
			return []byte{}
		}

		if err != nil {
			return []byte{}
		}

		encryptInput := kms.EncryptInput{
			KeyId:     aws.String(kMSKeyARN),
			Plaintext: data,
		}

		var encryptOutput *kms.EncryptOutput
		if encryptOutput, err = kmsSvc.Encrypt(&encryptInput); err != nil {
			return []byte{}
		}
		return encryptOutput.CiphertextBlob
	}

	encryptedAuthTokens := &EncryptedAuthTokens{AuthTokens{
		Contents: encrypt(r.Contents),
	}}
	if err != nil {
		return nil, err
	}
	return encryptedAuthTokens, nil
}

func (r *EncryptedAuthTokens) WriteToDir(dirname string) error {
	authTokensPath := filepath.Join(dirname, "tokens.csv.enc")

	if err := ioutil.WriteFile(authTokensPath, r.Contents, 0600); err != nil {
		return err
	}

	return nil
}

func (r *AuthTokens) Compact() (*CompactAuthTokens, error) {
	var err error
	compact := func(data []byte) string {
		if len(data) == 0 {
			return ""
		}

		if err != nil {
			return ""
		}

		var out string
		if out, err = gzipcompressor.CompressData(data); err != nil {
			return ""
		}
		return out
	}

	compactAuthTokens := &CompactAuthTokens{
		Contents: compact(r.Contents),
	}
	if err != nil {
		return nil, err
	}
	return compactAuthTokens, nil
}

func ReadOrCreateEncryptedAuthTokens(dirname string, kmsConfig KMSConfig) (*EncryptedAuthTokens, error) {
	var kmsSvc EncryptService

	encryptedTokens, err := ReadEncryptedAuthTokens(dirname)
	if err != nil {
		rawAuthTokens, err := ReadRawAuthTokens(dirname)
		if err != nil {
			return nil, err
		}

		awsConfig := aws.NewConfig().
			WithRegion(kmsConfig.Region.String()).
			WithCredentialsChainVerboseErrors(true)

		// TODO Cleaner way to inject this dependency
		if kmsConfig.EncryptService == nil {
			kmsSvc = kms.New(session.New(awsConfig))
		} else {
			kmsSvc = kmsConfig.EncryptService
		}

		encryptedTokens, err = rawAuthTokens.Encrypt(kmsConfig.KMSKeyARN, kmsSvc)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt auth tokens: %v", err)
		}

		// The fact KMS encryption produces different ciphertexts for the same plaintext had been
		// causing unnecessary node replacements(https://github.com/coreos/kube-aws/issues/107)
		// Write encrypted tls assets for caching purpose so that we can avoid that.
		encryptedTokens.WriteToDir(dirname)
	}

	return encryptedTokens, nil

}

func ReadOrCreateCompactAuthTokens(dirname string, kmsConfig KMSConfig) (*CompactAuthTokens, error) {
	encryptedTokens, err := ReadOrCreateEncryptedAuthTokens(dirname, kmsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to read/create auth token assets: %v", err)
	}

	compactTokens, err := encryptedTokens.Compact()
	if err != nil {
		return nil, fmt.Errorf("failed to compress auth token assets: %v", err)
	}

	return compactTokens, nil
}

func ReadOrCreateUnecryptedCompactAuthTokens(dirname string) (*CompactAuthTokens, error) {
	unencryptedTokens, err := ReadRawAuthTokens(dirname)
	if err != nil {
		return nil, fmt.Errorf("failed to read/create auth token assets: %v", err)
	}

	compactTokens, err := unencryptedTokens.Compact()
	if err != nil {
		return nil, fmt.Errorf("failed to compress auth token assets: %v", err)
	}

	return compactTokens, nil
}

func (t *CompactAuthTokens) HasTokens() bool {
	return len(t.Contents) > 0
}
