package pluginvalue

import (
	"github.com/kubernetes-incubator/kube-aws/plugin/pluginapi"
	"github.com/kubernetes-incubator/kube-aws/plugin/pluginutil"
)

type TemplateRenderer struct {
	p      *pluginapi.Plugin
	values interface{}
}

func TemplateRendererFor(p *pluginapi.Plugin, values interface{}) *TemplateRenderer {
	return &TemplateRenderer{
		p:      p,
		values: values,
	}
}

func (r *TemplateRenderer) StringFrom(expr string) (string, error) {
	return pluginutil.RenderStringFromTemplateWithValues(expr, r.values)
}
