package plugincontents

import (
	"encoding/json"
	"fmt"

	"bytes"
	"github.com/kubernetes-incubator/kube-aws/filereader/texttemplate"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"github.com/kubernetes-incubator/kube-aws/provisioner"
	"text/template"
)

type data struct {
	Config interface{}
	Values interface{}
}

func RenderStringFromTemplateWithValues(expr string, values interface{}, config interface{}) (string, error) {
	t, err := texttemplate.Parse("template", expr, template.FuncMap{})
	data := data{
		Values: values,
		Config: config,
	}
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %v", err)
	}

	var buff bytes.Buffer
	if err := t.Execute(&buff, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %v", err)
	}
	return buff.String(), nil
}

type TemplateRenderer struct {
	p      *api.Plugin
	l      *PluginFileLoader
	values interface{}
	config interface{}
}

func NewTemplateRenderer(p *api.Plugin, values interface{}, config interface{}) *TemplateRenderer {
	return &TemplateRenderer{
		p:      p,
		l:      NewPluginFileLoader(p),
		values: values,
		config: config,
	}
}

func (r *TemplateRenderer) File(f provisioner.RemoteFileSpec) (string, error) {
	str, err := r.l.String(f)
	if err != nil {
		return "", fmt.Errorf("failed to render template: %v", err)
	}
	if f.Type == "credential" {
		return str, nil
	}
	return RenderStringFromTemplateWithValues(str, r.values, r.config)
}

func (r *TemplateRenderer) String(str string) (string, error) {
	return RenderStringFromTemplateWithValues(str, r.values, r.config)
}

func (r *TemplateRenderer) MapFromJsonContents(contents provisioner.RemoteFileSpec) (map[string]interface{}, error) {
	str, err := r.File(contents)
	if err != nil {
		return nil, fmt.Errorf("failed to execute template: %v", err)
	}

	if len(str) == 0 {
		return map[string]interface{}{}, nil
	}

	m := map[string]interface{}{}
	if err := json.Unmarshal([]byte(str), &m); err != nil {
		return nil, fmt.Errorf("failed to parse json %s: %v", str, err)
	}

	return m, nil
}
