package config

import (
	"fmt"
	"github.com/coreos/kube-aws/coreos/userdatavalidation"
	"github.com/coreos/kube-aws/filereader/jsontemplate"
	"net/url"
)

type StackConfig struct {
	*Config
	StackTemplateOptions
	UserDataWorker        string
	UserDataController    string
	UserDataEtcd          string
	ControllerSubnetIndex int
}

type CompressedStackConfig struct {
	*StackConfig
}

func (c *StackConfig) UserDataControllerS3Path() (string, error) {
	s3uri, err := url.Parse(c.S3URI)
	if err != nil {
		return "", fmt.Errorf("Error in UserDataControllerS3Path : %v", err)
	}
	return fmt.Sprintf("%s%s/%s/userdata-controller", s3uri.Host, s3uri.Path, c.StackName()), nil
}

func (c *StackConfig) UserDataEtcdS3Path() (string, error) {
	s3uri, err := url.Parse(c.S3URI)
	if err != nil {
		return "", fmt.Errorf("Error in UserDataEtcdS3Path : %v", err)
	}
	return fmt.Sprintf("%s%s/%s/userdata-etcd", s3uri.Host, s3uri.Path, c.StackName()), nil
}

func (c *StackConfig) ValidateUserData() error {
	err := userdatavalidation.Execute([]userdatavalidation.Entry{
		{Name: "UserDataWorker", Content: c.UserDataWorker},
		{Name: "UserDataController", Content: c.UserDataController},
		{Name: "UserDataEtcd", Content: c.UserDataEtcd},
	})

	return err
}

func (c *StackConfig) Compress() (*CompressedStackConfig, error) {
	var stackConfig CompressedStackConfig
	stackConfig.StackConfig = &(*c)
	return &stackConfig, nil
}

func (c *CompressedStackConfig) RenderStackTemplateAsBytes() ([]byte, error) {
	bytes, err := jsontemplate.GetBytes(c.StackTemplateTmplFile, *c, c.PrettyPrint)
	if err != nil {
		return []byte{}, err
	}

	return bytes, nil
}

func (c *CompressedStackConfig) RenderStackTemplateAsString() (string, error) {
	bytes, err := c.RenderStackTemplateAsBytes()
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
