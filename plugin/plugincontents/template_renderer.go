package plugincontents

import (
	"encoding/json"
	"fmt"

	"github.com/kubernetes-incubator/kube-aws/plugin/pluginmodel"
	"github.com/kubernetes-incubator/kube-aws/plugin/pluginutil"
)

type TemplateRenderer struct {
	p      *pluginmodel.Plugin
	l      *Loader
	values interface{}
}

func TemplateRendererFor(p *pluginmodel.Plugin, values interface{}) *TemplateRenderer {
	return &TemplateRenderer{
		p:      p,
		l:      LoaderFor(p),
		values: values,
	}
}

func (r *TemplateRenderer) StringFrom(contents pluginmodel.Contents) (string, error) {
	str, err := r.l.StringFrom(contents)
	if err != nil {
		return "", fmt.Errorf("failed to render template: %v", err)
	}
	return pluginutil.RenderStringFromTemplateWithValues(str, r.values)
}

func (r *TemplateRenderer) MapFromContents(contents pluginmodel.Contents) (map[string]interface{}, error) {
	str, err := r.StringFrom(contents)
	if err != nil {
		return nil, fmt.Errorf("failed to execute template: %v", err)
	}

	m := map[string]interface{}{}
	if err := json.Unmarshal([]byte(str), &m); err != nil {
		return nil, fmt.Errorf("failed to parse json %s: %v", str, err)
	}

	return m, nil
}
