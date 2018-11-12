package api

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/kubernetes-incubator/kube-aws/filereader/texttemplate"
	"github.com/kubernetes-incubator/kube-aws/gzipcompressor"
	"github.com/kubernetes-incubator/kube-aws/logger"
)

type CustomFile struct {
	Path        string `yaml:"path"`
	Permissions uint   `yaml:"permissions"`
	Content     string `yaml:"content,omitempty"`
	Template    string `yaml:"template,omitempty"`
	Type        string `yaml:"type,omitempty"`
	UnknownKeys `yaml:",inline"`
}

func (c CustomFile) Encrypted() bool {
	return c.Type == "credential"
}

func (c CustomFile) PermissionsString() string {
	// We also need to write out octal notation for permissions.
	return fmt.Sprintf("0%o", c.Permissions)
}

func (c CustomFile) GzippedBase64Content() string {
	out, err := gzipcompressor.StringToGzippedBase64String(c.Content)
	if err != nil {
		return ""
	}
	return out
}

func (c CustomFile) RenderContent(ctx interface{}) (string, error) {
	var err error
	if c.customFileHasTemplate() {
		c.Content, err = c.renderTemplate(ctx)
		if err != nil {
			return "", err
		}
	}
	return c.Content, nil
}

func (c CustomFile) RenderGzippedBase64Content(ctx interface{}) (string, error) {
	var content string
	// Every credential is already encrypted by AWS KMS that its ciphertext is already a base64-encoded string.
	if c.Type == "credential" {
		content = c.Content
	} else {
		var err error
		content, err = c.RenderContent(ctx)
		if err != nil {
			return "", err
		}
	}
	return gzipcompressor.StringToGzippedBase64String(content)
}

func (c CustomFile) customFileHasTemplate() bool {
	return c.Template != ""
}

func (c CustomFile) renderTemplate(ctx interface{}) (string, error) {
	var buf strings.Builder

	tmpl, err := texttemplate.Parse("template", c.Template, template.FuncMap{})
	if err != nil {
		return "", fmt.Errorf("failed to parse CustomFile template %s: %v", c.Path, err)
	}
	err = tmpl.Execute(&buf, ctx)
	if err != nil {
		return "", fmt.Errorf("error rendering CustomFile template %s: %v", c.Path, err)
	}

	logger.Debugf("successfully rendered CustomFile template %s", c.Path)
	return buf.String(), nil
}
