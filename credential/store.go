package credential

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"os"
)

func (e Store) EncryptedCredentialFromPath(filePath string, defaultValue *string) (*EncryptedFile, error) {
	raw, errRaw := RawCredentialFileFromPath(filePath, defaultValue)
	cache, err := EncryptedCredentialCacheFromPath(filePath, errRaw == nil)
	if err != nil {
		if errRaw != nil { // if neither .enc nor raw is there, it is an error
			return nil, fmt.Errorf("Error reading raw file: %v", errRaw)
		}
		cache, err = EncryptedCredentialCacheFromRawCredential(raw, e.Encryptor)
		if err != nil {
			return nil, err
		}
		logger.Infof("generated \"%s\" by encrypting \"%s\"\n", cache.filePath, raw.filePath)
	} else {
		// we verify fingreprints only if non .enc version is present, so there is something there to compare against
		// otherwise we assume that user provided correct .enc files to be used as-is
		if errRaw == nil && raw.Fingerprint() != cache.Fingerprint() {
			logger.Infof("\"%s\" is not up-to-date. kube-aws is regenerating it from \"%s\"\n", cache.filePath, raw.filePath)
			cache, err = EncryptedCredentialCacheFromRawCredential(raw, e.Encryptor)
			if err != nil {
				return nil, err
			}
		} else if errRaw != nil && !os.IsNotExist(errRaw) {
			return nil, fmt.Errorf("Error reading existing raw file: %v", errRaw)
		}
	}

	return cache, nil
}
