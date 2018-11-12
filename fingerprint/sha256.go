package fingerprint

import (
	"crypto/sha256"
	"fmt"
)

// SHA256 calculates and returns a SHA-256 hash intended to be used for a fingerprint of the original data
func SHA256(data string) string {
	h := sha256.New()
	h.Write([]byte(data))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func BytesToSha256(data []byte) []byte {
	h := sha256.New()
	h.Write([]byte(data))
	return []byte(fmt.Sprintf("%x", h.Sum(nil)))
}
