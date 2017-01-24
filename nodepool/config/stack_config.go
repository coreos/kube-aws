package config

import (
	"fmt"
	"github.com/coreos/kube-aws/coreos/userdatavalidation"
	"github.com/coreos/kube-aws/filereader/jsontemplate"
	"net/url"
)

type StackConfig struct {
	*ComputedConfig
	UserDataWorker string
	StackTemplateOptions
}

type CompressedStackConfig struct {
	*StackConfig
}

func (c *StackConfig) UserDataWorkerS3Path() (string, error) {
	s3uri, err := url.Parse(c.S3URI)
	if err != nil {
		return "", fmt.Errorf("Error in UserDataWorkerS3Path : %v", err)
	}
	return fmt.Sprintf("%s%s/%s/userdata-worker", s3uri.Host, s3uri.Path, c.StackName()), nil
}

func (c *StackConfig) ValidateUserData() error {
	err := userdatavalidation.Execute([]userdatavalidation.Entry{
		{Name: "UserDataWorker", Content: c.UserDataWorker},
	})

	return err
}

func (c *StackConfig) Compress() (*CompressedStackConfig, error) {
	//var err error
	//var compressedWorkerUserData string
	//
	//if compressedWorkerUserData, err = gzipcompressor.CompressString(c.UserDataWorker); err != nil {
	//	return nil, err
	//}

	var stackConfig CompressedStackConfig
	stackConfig.StackConfig = &(*c)
	//stackConfig.UserDataWorker = compressedWorkerUserData
	stackConfig.UserDataWorker = c.UserDataWorker

	return &stackConfig, nil
}

func (c *CompressedStackConfig) RenderStackTemplateAsBytes() ([]byte, error) {
	bytes, err := jsontemplate.GetBytes(c.StackTemplateTmplFile, *c, c.PrettyPrint)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to render : %v", err)
	}

	return bytes, nil
}

func (c *CompressedStackConfig) RenderStackTemplateAsString() (string, error) {
	bytes, err := c.RenderStackTemplateAsBytes()
	if err != nil {
		return "", fmt.Errorf("failed to render to str : %v", err)
	}
	return string(bytes), nil
}
