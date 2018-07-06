package model

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/kubernetes-incubator/kube-aws/gzipcompressor"
	"github.com/kubernetes-incubator/kube-aws/logger"
)

type CustomFile struct {
	Path        string `yaml:"path"`
	Permissions uint   `yaml:"permissions"`
	Content     string `yaml:"content,omitempty"`
	Template    string `yaml:"template,omitempty"`
	UnknownKeys `yaml:",inline"`
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
	content, err := c.RenderContent(ctx)
	if err != nil {
		return "", err
	}
	return gzipcompressor.CompressString(content)
}

func (c CustomFile) customFileHasTemplate() bool {
	return c.Template != ""
}

func (c CustomFile) renderTemplate(ctx interface{}) (string, error) {
	var buf strings.Builder

	tmpl, err := template.New("").Parse(c.Template)
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
