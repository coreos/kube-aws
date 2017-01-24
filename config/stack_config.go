package config

import (
	"fmt"
	"github.com/coreos/kube-aws/coreos/userdatavalidation"
	"github.com/coreos/kube-aws/filereader/jsontemplate"
	"github.com/coreos/kube-aws/gzipcompressor"
	"net/url"
)

type StackConfig struct {
	*Config
	StackTemplateOptions
	UserDataWorker        string
	UserDataController    string
	userDataEtcd          string
	ControllerSubnetIndex int
}

type CompressedStackConfig struct {
	*StackConfig
	UserDataEtcd string
}

func (c *StackConfig) UserDataWorkerS3Path() (string, error) {
	s3uri, err := url.Parse(c.S3URI)
	if err != nil {
		return "", fmt.Errorf("Error in UserDataWorkerS3Path : %v", err)
	}
	return fmt.Sprintf("%s%s/%s/userdata-worker", s3uri.Host, s3uri.Path, c.StackName()), nil
}

func (c *StackConfig) UserDataControllerS3Path() (string, error) {
	s3uri, err := url.Parse(c.S3URI)
	if err != nil {
		return "", fmt.Errorf("Error in UserDataControllerS3Path : %v", err)
	}
	return fmt.Sprintf("%s%s/%s/userdata-controller", s3uri.Host, s3uri.Path, c.StackName()), nil
}

func (c *StackConfig) ValidateUserData() error {
	err := userdatavalidation.Execute([]userdatavalidation.Entry{
		{Name: "UserDataWorker", Content: c.UserDataWorker},
		{Name: "UserDataController", Content: c.UserDataController},
		{Name: "UserDataEtcd", Content: c.userDataEtcd},
	})

	return err
}

func (c *StackConfig) Compress() (*CompressedStackConfig, error) {
	var err error
	var compressedEtcdUserData string

	if compressedEtcdUserData, err = gzipcompressor.CompressString(c.userDataEtcd); err != nil {
		return nil, err
	}

	var stackConfig CompressedStackConfig
	stackConfig.StackConfig = &(*c)
	stackConfig.UserDataEtcd = compressedEtcdUserData

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
