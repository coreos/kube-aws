package config

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"os"
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
type RawAuthTokensOnDisk struct {
	AuthTokens RawCredentialOnDisk
}

// Encrypted contents of the CSV file holding auth tokens.
type EncryptedAuthTokensOnDisk struct {
	AuthTokens EncryptedCredentialOnDisk
}

// Encrypted -> gzip -> base64 encoded auth token file contents.
type CompactAuthTokens struct {
	Contents string
}

func NewAuthTokens() AuthTokens {
	// Uses an empty file as the default auth token file
	return AuthTokens{
		Contents: make([]byte, 0),
	}
}

func NewAuthTokensOnDisk(dir string) (*RawAuthTokensOnDisk, error) {
	authToken := NewAuthTokens()
	if err := authToken.WriteToDir(dir); err != nil {
		return nil, fmt.Errorf("error creating auth token file: %v", err)
	}
	return ReadRawAuthTokens(dir)
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

func ReadRawAuthTokens(dirname string) (*RawAuthTokensOnDisk, error) {
	authTokensPath := filepath.Join(dirname, "tokens.csv")

	data, err := RawCredentialFileFromPath(authTokensPath)
	if err != nil {
		return nil, err
	}

	authTokens := data.content
	if err = validateAuthTokens(authTokens); err != nil {
		return nil, err
	}

	return &RawAuthTokensOnDisk{AuthTokens: *data}, nil
}

func (r AuthTokens) WriteToDir(dirname string) error {
	authTokensPath := filepath.Join(dirname, "tokens.csv")

	if err := ioutil.WriteFile(authTokensPath, r.Contents, 0600); err != nil {
		return err
	}

	return nil
}

func (r *RawAuthTokensOnDisk) Compact() (*CompactAuthTokens, error) {
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
		Contents: compact(r.AuthTokens.content),
	}
	if err != nil {
		return nil, err
	}
	return compactAuthTokens, nil
}

func (r *EncryptedAuthTokensOnDisk) Compact() (*CompactAuthTokens, error) {
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
		Contents: compact(r.AuthTokens.content),
	}
	if err != nil {
		return nil, err
	}
	return compactAuthTokens, nil
}

func ReadOrEncryptAuthTokens(dirname string, encryptor CachedEncryptor) (*EncryptedAuthTokensOnDisk, error) {
	authTokenPath := filepath.Join(dirname, "tokens.csv")

	// Auto-creates the auth token file, useful for those coming from previous versions of kube-aws
	if _, err := os.Stat(authTokenPath); os.IsNotExist(err) {
		file, err := os.OpenFile(authTokenPath, os.O_RDONLY|os.O_CREATE, 0600)
		if err != nil {
			return nil, err
		}
		file.Close()
	}

	if _, err := ReadRawAuthTokens(dirname); err != nil {
		return nil, err
	}

	data, err := encryptor.EncryptedCredentialFromPath(authTokenPath)
	if err != nil {
		return nil, err
	}
	if err := data.Persist(); err != nil {
		return nil, err
	}
	return &EncryptedAuthTokensOnDisk{
		AuthTokens: *data,
	}, nil
}

func ReadOrCreateEncryptedAuthTokens(dirname string, kmsConfig KMSConfig) (*EncryptedAuthTokensOnDisk, error) {
	var kmsSvc EncryptService

	awsConfig := aws.NewConfig().
		WithRegion(kmsConfig.Region.String()).
		WithCredentialsChainVerboseErrors(true)

	// TODO Cleaner way to inject this dependency
	if kmsConfig.EncryptService == nil {
		kmsSvc = kms.New(session.New(awsConfig))
	} else {
		kmsSvc = kmsConfig.EncryptService
	}

	encryptor := CachedEncryptor{
		bytesEncryptionService: bytesEncryptionService{
			kmsKeyARN: kmsConfig.KMSKeyARN,
			kmsSvc:    kmsSvc,
		},
	}

	return ReadOrEncryptAuthTokens(dirname, encryptor)
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
