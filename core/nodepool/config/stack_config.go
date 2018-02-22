package config

import (
	"github.com/kubernetes-incubator/kube-aws/filereader/jsontemplate"
	"github.com/kubernetes-incubator/kube-aws/model"
)

type StackConfig struct {
	*ComputedConfig
	UserDataWorker model.UserData
	StackTemplateOptions
	ExtraCfnResources map[string]interface{}
}

func (c *StackConfig) RenderStackTemplateAsBytes() ([]byte, error) {
	return jsontemplate.GetBytes(c.StackTemplateTmplFile, *c, c.PrettyPrint)
}

func (c *StackConfig) RenderStackTemplateAsString() (string, error) {
	bytes, err := c.RenderStackTemplateAsBytes()
	return string(bytes), err
}
