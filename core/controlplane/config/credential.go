package config

import (
	"crypto/sha256"
	"fmt"
	"io/ioutil"
)

const CacheFileExtension = "enc"
const FingerprintFileExtension = "fingerprint"

type RawCredentialOnDisk struct {
	content  []byte
	filePath string
}

// The fact KMS encryption produces different ciphertexts for the same plaintext had been
// causing unnecessary node replacements(https://github.com/kubernetes-incubator/kube-aws/issues/107)
// Persist encrypted assets for caching purpose so that we can avoid that.
type EncryptedCredentialOnDisk struct {
	content             []byte
	filePath            string
	fingerprintFilePath string
	fingerprint         string
}

type CachedEncryptor struct {
	bytesEncryptionService bytesEncryptionService
}

func (e CachedEncryptor) EncryptedBytes(raw []byte) ([]byte, error) {
	return e.bytesEncryptionService.Encrypt(raw)
}

func (e CachedEncryptor) EncryptedCredentialFromPath(filePath string) (*EncryptedCredentialOnDisk, error) {
	raw, err := RawCredentialFileFromPath(filePath)
	if err != nil {
		return nil, err
	}

	cache, err := EncryptedCredentialCacheFromPath(filePath)
	if err != nil {
		cache, err = EncryptedCredentialCacheFromRawCredential(raw, e.bytesEncryptionService)
		if err != nil {
			return nil, err
		}
		fmt.Printf("INFO: generated \"%s\" by encrypting \"%s\"\n", cache.filePath, raw.filePath)
	} else if raw.Fingerprint() != cache.Fingerprint() {
		fmt.Printf("INFO: \"%s\" is not up-to-date. kube-aws is regenerating it from \"%s\"\n", cache.filePath, raw.filePath)
		cache, err = EncryptedCredentialCacheFromRawCredential(raw, e.bytesEncryptionService)
		if err != nil {
			return nil, err
		}
	}

	return cache, nil
}

func EncryptedCredentialCacheFromRawCredential(raw *RawCredentialOnDisk, bytesEncryptionService bytesEncryptionService) (*EncryptedCredentialOnDisk, error) {
	encrypted, err := bytesEncryptionService.Encrypt(raw.content)
	if err != nil {
		return nil, err
	}
	cache := EncryptedCredentialOnDisk{
		filePath:            cacheFilePath(raw.filePath),
		fingerprintFilePath: fingerprintFilePath(raw.filePath),
		content:             encrypted,
		fingerprint:         raw.Fingerprint(),
	}
	if err := cache.Persist(); err != nil {
		return nil, err
	}
	return &cache, nil
}

func RawCredentialFileFromPath(filePath string) (*RawCredentialOnDisk, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return &RawCredentialOnDisk{
		filePath: filePath,
		content:  content,
	}, nil
}

func (c *RawCredentialOnDisk) Fingerprint() string {
	return calculateFingerprint(c.content)
}

func (c *RawCredentialOnDisk) Persist() error {
	if err := ioutil.WriteFile(c.filePath, c.content, 0600); err != nil {
		return err
	}
	return nil
}

func (c *RawCredentialOnDisk) String() string {
	return string(c.content)
}

func cacheFilePath(rawCredFilePath string) string {
	return fmt.Sprintf("%s.%s", rawCredFilePath, CacheFileExtension)
}

func fingerprintFilePath(rawCredFilePath string) string {
	return fmt.Sprintf("%s.%s", rawCredFilePath, FingerprintFileExtension)
}

func EncryptedCredentialCacheFromPath(filePath string) (*EncryptedCredentialOnDisk, error) {
	cachePath := cacheFilePath(filePath)
	credential, cacheErr := ioutil.ReadFile(cachePath)
	if cacheErr != nil {
		return nil, cacheErr
	}
	fingerprintPath := fingerprintFilePath(filePath)
	fingerprint, fingerprintErr := loadFingerprint(fingerprintPath)
	if fingerprintErr != nil {
		fmt.Printf("WARNING: \"%s\" does not exist. Did you explicitly removed it or upgrading from old kube-aws? Anyway, kube-aws is generating one for you from \"%s\" to automatically detect updates to it and recreate \"%s\" if necessary\n", fingerprintPath, filePath, cachePath)
		raw, rawErr := RawCredentialFileFromPath(filePath)
		if rawErr != nil {
			return nil, rawErr
		}
		fingerprint = raw.Fingerprint()
	}
	return &EncryptedCredentialOnDisk{
		filePath:            cachePath,
		fingerprintFilePath: fingerprintPath,
		content:             credential,
		fingerprint:         fingerprint,
	}, nil
}

func (c *EncryptedCredentialOnDisk) Fingerprint() string {
	return c.fingerprint
}

func (c *EncryptedCredentialOnDisk) Persist() error {
	if err := ioutil.WriteFile(c.filePath, c.content, 0600); err != nil {
		return err
	}
	if err := ioutil.WriteFile(c.fingerprintFilePath, []byte(c.fingerprint), 0600); err != nil {
		return err
	}
	return nil
}

func (c *EncryptedCredentialOnDisk) String() string {
	return string(c.content)
}

func loadFingerprint(file string) (string, error) {
	p, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(p), nil
}

func calculateFingerprint(content []byte) string {
	h := sha256.New()
	h.Write(content)
	return fmt.Sprintf("%x", h.Sum(nil))
}
