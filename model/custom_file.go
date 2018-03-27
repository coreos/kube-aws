package model

import (
	"fmt"

	"github.com/kubernetes-incubator/kube-aws/gzipcompressor"
)

type CustomFile struct {
	Path        string `yaml:"path"`
	Permissions uint   `yaml:"permissions"`
	Content     string `yaml:"content"`
}

func (c CustomFile) PermissionsString() string {
	// We also need to write out octal notation for permissions.
	return fmt.Sprintf("0%o", c.Permissions)
}

func (c CustomFile) GzippedBase64Content() string {
	out, err := gzipcompressor.CompressString(c.Content)
	if err != nil {
		return ""
	}
	return out
}
