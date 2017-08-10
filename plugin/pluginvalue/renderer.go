package pluginvalue

import (
	"github.com/kubernetes-incubator/kube-aws/plugin/pluginmodel"
	"github.com/kubernetes-incubator/kube-aws/plugin/pluginutil"
)

type TemplateRenderer struct {
	p      *pluginmodel.Plugin
	values interface{}
}

func TemplateRendererFor(p *pluginmodel.Plugin, values interface{}) *TemplateRenderer {
	return &TemplateRenderer{
		p:      p,
		values: values,
	}
}

func (r *TemplateRenderer) StringFrom(expr string) (string, error) {
	return pluginutil.RenderStringFromTemplateWithValues(expr, r.values)
}
