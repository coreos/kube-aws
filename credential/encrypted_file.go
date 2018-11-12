package credential

import "io/ioutil"

func (c *EncryptedFile) Fingerprint() string {
	return c.fingerprint
}

func (c *EncryptedFile) Persist() error {
	if err := ioutil.WriteFile(c.filePath, c.content, 0600); err != nil {
		return err
	}
	if c.fingerprint != "" {
		return ioutil.WriteFile(c.fingerprintFilePath, []byte(c.fingerprint), 0600)
	}
	return nil
}

func (c *EncryptedFile) String() string {
	return string(c.content)
}

func (c *EncryptedFile) Bytes() []byte {
	return c.content
}

func (c *EncryptedFile) SetBytes(bytes []byte) {
	c.content = bytes
}
