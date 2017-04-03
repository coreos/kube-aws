package config

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"

	"github.com/kubernetes-incubator/kube-aws/gzipcompressor"
)

const (
	authTokenFilename = "tokens.csv"

	// Taken from https://kubernetes.io/docs/admin/kubelet-tls-bootstrapping/#apiserver-configuration
	kubeletBootstrapGroup     = "system:kubelet-bootstrap"
	kubeletBootstrapUser      = "kubelet-bootstrap"
	kubeletBootstrapUserId    = "10001"
	kubeletBootstrapTokenBits = 256
)

type RawAuthTokensOnMemory struct {
	// Contents of the CSV file holding auth tokens.
	Contents []byte
}

type RawAuthTokensOnDisk struct {
	// Contents of the CSV file holding auth tokens.
	AuthTokens RawCredentialOnDisk

	// Extracted from the auth tokens file
	KubeletBootstrapToken []byte
}

type EncryptedAuthTokensOnDisk struct {
	// Encrypted contents of the CSV file holding auth tokens.
	AuthTokens EncryptedCredentialOnDisk

	// Encrypted version of the Kubelet bootstrap token.
	KubeletBootstrapToken []byte
}

type CompactAuthTokens struct {
	// Encrypted -> gzip -> base64 encoded auth token file contents.
	Contents string

	// Encrypted -> gzip -> base64 encoded version of the Kubelet auth token.
	KubeletBootstrapToken string
}

func parseAuthTokensCSV(authTokens []byte) ([][]string, error) {
	if len(authTokens) == 0 {
		return make([][]string, 0), nil
	}

	csvReader := csv.NewReader(bytes.NewReader(authTokens))
	return csvReader.ReadAll()
}

func NewAuthTokens() RawAuthTokensOnMemory {
	// Uses an empty file as the default auth token file
	return RawAuthTokensOnMemory{
		Contents: make([]byte, 0),
	}
}

func AuthTokensFileExists(dirname string) bool {
	authTokensPath := filepath.Join(dirname, authTokenFilename)
	stat, err := os.Stat(authTokensPath)

	// Considers empty token file as non-existent
	if os.IsNotExist(err) || stat.Size() == 0 {
		return false
	}

	return true
}

func RandomKubeletBootstrapTokenString(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func RandomBootstrapTokenRecord() (string, error) {
	randomToken, err := RandomKubeletBootstrapTokenString(kubeletBootstrapTokenBits)
	if err != nil {
		return "", fmt.Errorf("cannot generate a random Kubelet bootstrap token: %v", err)
	}
	return fmt.Sprintf("%s,%s,%s,%s", randomToken, kubeletBootstrapUser, kubeletBootstrapUserId, kubeletBootstrapGroup), nil
}

func CreateRawAuthTokens(addBootstrapToken bool, dirname string) (bool, error) {
	tokens := RawAuthTokensOnMemory{}

	if addBootstrapToken {
		bootstrapToken, err := RandomBootstrapTokenRecord()
		if err != nil {
			return false, err
		}
		tokens.Contents = []byte(bootstrapToken)
		if err = tokens.WriteToDir(dirname); err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}

func KubeletBootstrapTokenFromRecord(csvRecord []string) (string, error) {
	if csvRecord == nil || len(csvRecord) < 4 {
		return "", nil
	}

	pattern := fmt.Sprintf(`^(?:\s*\S+\s*,\s*|\s+)?(?:%s)(?:\s*|\s*,\s*\S+\s*)+$`, kubeletBootstrapGroup)
	match, err := regexp.MatchString(pattern, csvRecord[3])
	if err != nil {
		return "", fmt.Errorf("error trying to identify the Kubelet bootstrap token")
	}

	if match {
		return csvRecord[0], nil
	}

	return "", nil
}

func ReadRawAuthTokens(dirname string) (*RawAuthTokensOnDisk, error) {
	authTokensPath := filepath.Join(dirname, authTokenFilename)
	kubeletBootstrapToken := []byte{}

	// Ignore if the auth token file does not exist
	if !AuthTokensFileExists(dirname) {
		return &RawAuthTokensOnDisk{
			AuthTokens: RawCredentialOnDisk{
				content:  []byte{},
				filePath: authTokensPath,
			},
			KubeletBootstrapToken: kubeletBootstrapToken,
		}, nil
	}

	data, err := RawCredentialFileFromPath(authTokensPath)
	if err != nil {
		return nil, err
	}

	records, err := parseAuthTokensCSV(data.content)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth token file: %v", err)
	}

	// Checks whether the CSV file has the expected layout
	for _, line := range records {
		columns := len(line)
		if columns < 3 {
			return nil, fmt.Errorf("auth token record must have at least 3 columns, but has %d: '%v'", columns, line)
		}

		// Keeps the first token assigned for the Kubelet bootstrap group
		if len(kubeletBootstrapToken) == 0 && columns > 3 {
			kubeletBootstrapTokenFromRecord, err := KubeletBootstrapTokenFromRecord(line)
			if err != nil {
				return nil, err
			}

			if len(kubeletBootstrapTokenFromRecord) > 0 {
				kubeletBootstrapToken = []byte(kubeletBootstrapTokenFromRecord)
			}
		}
	}

	return &RawAuthTokensOnDisk{
		AuthTokens:            *data,
		KubeletBootstrapToken: kubeletBootstrapToken,
	}, nil
}

func (r RawAuthTokensOnMemory) WriteToDir(dirname string) error {
	authTokensPath := filepath.Join(dirname, authTokenFilename)

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
		Contents:              compact(r.AuthTokens.content),
		KubeletBootstrapToken: compact(r.KubeletBootstrapToken),
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
		Contents:              compact(r.AuthTokens.content),
		KubeletBootstrapToken: compact(r.KubeletBootstrapToken),
	}
	if err != nil {
		return nil, err
	}
	return compactAuthTokens, nil
}

func ReadOrEncryptAuthTokens(dirname string, encryptor CachedEncryptor) (*EncryptedAuthTokensOnDisk, error) {
	authTokensPath := filepath.Join(dirname, authTokenFilename)

	// Ignore if the auth token file does not exist
	if !AuthTokensFileExists(dirname) {
		return &EncryptedAuthTokensOnDisk{
			AuthTokens: EncryptedCredentialOnDisk{
				content: []byte{},
			},
			KubeletBootstrapToken: []byte{},
		}, nil
	}

	// Extracts and encrypts the Kubelet bootstrap token
	authTokens, err := ReadRawAuthTokens(dirname)
	if err != nil {
		return nil, err
	}
	encryptedToken, err := encryptor.EncryptedBytes(authTokens.KubeletBootstrapToken)
	if err != nil {
		return nil, err
	}

	data, err := encryptor.EncryptedCredentialFromPath(authTokensPath)
	if err != nil {
		return nil, err
	}
	if err := data.Persist(); err != nil {
		return nil, err
	}

	return &EncryptedAuthTokensOnDisk{
		AuthTokens:            *data,
		KubeletBootstrapToken: encryptedToken,
	}, nil
}

func ReadOrCreateEncryptedAuthTokens(dirname string, kmsConfig KMSConfig) (*EncryptedAuthTokensOnDisk, error) {
	var kmsSvc EncryptService

	// TODO Cleaner way to inject this dependency
	if kmsConfig.EncryptService == nil {
		awsConfig := aws.NewConfig().
			WithRegion(kmsConfig.Region.String()).
			WithCredentialsChainVerboseErrors(true)
		kmsSvc = kms.New(session.New(awsConfig))
	} else {
		kmsSvc = kmsConfig.EncryptService
	}

	encryptionSvc := bytesEncryptionService{
		kmsKeyARN: kmsConfig.KMSKeyARN,
		kmsSvc:    kmsSvc,
	}

	encryptor := CachedEncryptor{
		bytesEncryptionService: encryptionSvc,
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

func ReadOrCreateUnencryptedCompactAuthTokens(dirname string) (*CompactAuthTokens, error) {
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
