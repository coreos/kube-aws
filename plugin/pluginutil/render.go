package pluginutil

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/kubernetes-incubator/kube-aws/filereader/texttemplate"
)

func RenderStringFromTemplateWithValues(expr string, values interface{}) (string, error) {
	t, err := texttemplate.Parse("template", expr, template.FuncMap{})
	data := map[string]interface{}{
		"Values": values,
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
