package credential

import (
	"crypto/sha256"
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"io/ioutil"
	"os"
	"regexp"
)

const CacheFileExtension = "enc"
const FingerprintFileExtension = "fingerprint"

func CreateEncryptedFile(path string, bytes []byte, svc Encryptor) (*EncryptedFile, error) {
	encrypted, err := svc.EncryptedBytes(bytes)
	if err != nil {
		return nil, err
	}
	cache := EncryptedFile{
		filePath:            cacheFilePath(path),
		fingerprintFilePath: fingerprintFilePath(path),
		content:             encrypted,
		fingerprint:         calculateFingerprint(bytes),
	}
	if err := cache.Persist(); err != nil {
		return nil, err
	}
	return &cache, nil
}

func EncryptedCredentialCacheFromRawCredential(raw *PlaintextFile, encSvc Encryptor) (*EncryptedFile, error) {
	return CreateEncryptedFile(raw.filePath, raw.content, encSvc)
}

func RawCredentialFileFromPath(filePath string, defaultValue *string) (*PlaintextFile, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if defaultValue == nil {
			return nil, fmt.Errorf("%s must exist. Please confirm that you have not deleted the file manually", filePath)
		}
		// special default value that allows lookup from another file
		re := regexp.MustCompile("^<<<([a-z./-]+.pem)$")
		if re.MatchString(*defaultValue) {
			readPath := re.FindStringSubmatch(*defaultValue)
			if _, err := os.Stat(readPath[1]); os.IsNotExist(err) {
				return nil, fmt.Errorf("%s and alternate file %s do not exist. Please confirm that you have not deleted them manually", filePath, readPath[1])
			}
			logger.Infof("creating \"%s\" with contents of \"%s\"\n", filePath, readPath[1])
			content, err := ioutil.ReadFile(readPath[1])
			if err != nil {
				return nil, err
			}
			newDefault := string(content[:])
			return RawCredentialFileFromPath(filePath, &newDefault)
		}
		if err := ioutil.WriteFile(filePath, []byte(*defaultValue), 0644); err != nil {
			return nil, err
		}
	}

	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return &PlaintextFile{
		filePath: filePath,
		content:  content,
	}, nil
}

func cacheFilePath(rawCredFilePath string) string {
	return fmt.Sprintf("%s.%s", rawCredFilePath, CacheFileExtension)
}

func fingerprintFilePath(rawCredFilePath string) string {
	return fmt.Sprintf("%s.%s", rawCredFilePath, FingerprintFileExtension)
}

func EncryptedCredentialCacheFromPath(filePath string, doLoadFingerprint bool) (*EncryptedFile, error) {
	cachePath := cacheFilePath(filePath)
	credential, cacheErr := ioutil.ReadFile(cachePath)
	if cacheErr != nil {
		return nil, cacheErr
	}

	fingerprintPath := fingerprintFilePath(filePath)
	var fingerprint string
	if doLoadFingerprint {
		var err error
		if fingerprint, err = loadFingerprint(fingerprintPath); err != nil {
			logger.Warnf("\"%s\" does not exist. Did you explicitly removed it or upgrading from old kube-aws? Anyway, kube-aws is generating one for you from \"%s\" to automatically detect updates to it and recreate \"%s\" if necessary\n", fingerprintPath, filePath, cachePath)
			raw, rawErr := RawCredentialFileFromPath(filePath, nil)
			if rawErr != nil {
				return nil, rawErr
			}

			fingerprint = raw.Fingerprint()
		}
	}
	return &EncryptedFile{
		filePath:            cachePath,
		fingerprintFilePath: fingerprintPath,
		content:             credential,
		fingerprint:         fingerprint,
	}, nil
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
