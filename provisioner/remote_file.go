package provisioner

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/filereader/texttemplate"
	"github.com/kubernetes-incubator/kube-aws/gzipcompressor"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"strings"
	"text/template"
)

func NewRemoteFile(spec RemoteFileSpec) *RemoteFile {
	var loaded RemoteFile
	loaded.Path = spec.Path
	loaded.Permissions = spec.Permissions
	loaded.Type = spec.Type

	if spec.Template != "" {
		loaded.Content = NewStringContent(spec.Template)
		loaded.Type = "template"
	}

	return &loaded
}

func NewRemoteFileWithContent(spec RemoteFileSpec, content []byte) *RemoteFile {
	return &RemoteFile{
		Path:        spec.Path,
		Content:     NewBinaryContent(content),
		Permissions: spec.Permissions,
		Type:        spec.Type,
	}
}

func NewRemoteFileAtPath(path string, content []byte) *RemoteFile {
	spec := RemoteFileSpec{
		Path: path,
	}
	return NewRemoteFileWithContent(spec, content)
}

func (c RemoteFile) PermissionsString() string {
	// We also need to write out octal notation for permissions.
	return fmt.Sprintf("0%o", c.Permissions)
}

func (c RemoteFile) GzippedBase64Content() string {
	return c.Content.GzippedBase64Content()
}

func (c RemoteFile) RenderContent(ctx interface{}) (string, error) {
	if c.hasTemplate() {
		bytes, err := c.renderTemplate(ctx)
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}
	return c.Content.String(), nil
}

func (c RemoteFile) RenderGzippedBase64Content(ctx interface{}) (string, error) {
	content, err := c.RenderContent(ctx)
	if err != nil {
		return "", err
	}
	return gzipcompressor.StringToGzippedBase64String(content)
}

func (c RemoteFile) hasTemplate() bool {
	return c.Type == "template"
}

func (c RemoteFile) renderTemplate(ctx interface{}) (string, error) {
	var buf strings.Builder

	tmpl, err := texttemplate.Parse("template", c.Content.String(), template.FuncMap{})
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
