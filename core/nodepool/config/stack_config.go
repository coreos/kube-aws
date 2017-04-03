package config

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/coreos/userdatavalidation"
	"github.com/kubernetes-incubator/kube-aws/filereader/jsontemplate"
	"github.com/kubernetes-incubator/kube-aws/fingerprint"
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

// UserDataWorkerS3Prefix is the prefix prepended to all user-data-worker-<fingerprint> files uploaded to S3
// Use this to author the IAM policy to provide worker nodes least required permissions for getting the files from S3
func (c *StackConfig) UserDataWorkerS3Prefix() (string, error) {
	s3dir, err := c.userDataWorkerS3Directory()
	if err != nil {
		return "", fmt.Errorf("Error in UserDataWorkerS3Prefix : %v", err)
	}
	return fmt.Sprintf("%s/userdata-worker", s3dir), nil
}

func (c *StackConfig) userDataWorkerS3Directory() (string, error) {
	s3uri, err := url.Parse(c.S3URI)
	if err != nil {
		return "", fmt.Errorf("Error in userDataWorkerS3Directory : %v", err)
	}
	return fmt.Sprintf("%s%s/%s", s3uri.Host, s3uri.Path, c.StackName()), nil
}

// UserDataWorkerS3URI is the URI to an userdata-worker-<fingerprint> file used to provision worker nodes
// Use this to run download the file by running e.g. `aws cp *return value of UserDataWorkerS3URI* ./`
func (c *StackConfig) UserDataWorkerS3URI() (string, error) {
	s3dir, err := c.userDataWorkerS3Directory()
	if err != nil {
		return "", fmt.Errorf("Error in UserDataWorkerS3URI : %v", err)
	}
	return fmt.Sprintf("s3://%s/%s", s3dir, c.UserDataWorkerFileName()), nil
}

// UserDataWorkerFileName is used to upload and download userdata-worker-<fingerprint> files
func (c *StackConfig) UserDataWorkerFileName() string {
	return "userdata-worker-" + fingerprint.SHA256(c.UserDataWorker)
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
