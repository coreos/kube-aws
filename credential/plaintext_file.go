package credential

import (
	"fmt"
	"io/ioutil"
)

func (c *PlaintextFile) Fingerprint() string {
	return calculateFingerprint(c.content)
}

func (c *PlaintextFile) Persist() error {
	if len(c.content) == 0 {
		return fmt.Errorf("%s is going to be empty. Maybe a bug", c.filePath)
	}
	if err := ioutil.WriteFile(c.filePath, c.content, 0600); err != nil {
		return err
	}
	return nil
}

func (c *PlaintextFile) String() string {
	return string(c.content)
}

func (c *PlaintextFile) Bytes() []byte {
	return c.content
}
