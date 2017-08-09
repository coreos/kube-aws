package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/kubernetes-incubator/kube-aws/filereader/jsontemplate"
	"github.com/kubernetes-incubator/kube-aws/model"
)

// StackConfig contains configuration parameters available when rendering CFN stack template from golang text templates
type StackConfig struct {
	*Config
	StackTemplateOptions
	UserDataController    model.UserData
	UserDataEtcd          model.UserData
	ControllerSubnetIndex int
	ExtraCfnResources     map[string]interface{}
}

func (c *StackConfig) s3Folders() model.S3Folders {
	return model.NewS3Folders(c.S3URI, c.ClusterName)
}

func (c *StackConfig) ClusterS3URI() string {
	return c.s3Folders().Cluster().URI()
}

func (c *StackConfig) ClusterExportedStacksS3URI() string {
	return c.s3Folders().ClusterExportedStacks().URI()
}

// EtcdSnapshotsS3Path is a pair of a S3 bucket and a key of an S3 object containing an etcd cluster snapshot
func (c StackConfig) EtcdSnapshotsS3PathRef() (string, error) {
	s3uri, err := url.Parse(c.ClusterS3URI())
	if err != nil {
		return "", fmt.Errorf("Error in EtcdSnapshotsS3PathRef : %v", err)
	}
	return fmt.Sprintf(`{ "Fn::Join" : [ "", [ "%s%s/instances/", { "Fn::Select" : [ "2", { "Fn::Split": [ "/", { "Ref": "AWS::StackId" }]} ]}, "/etcd-snapshots" ]]}`, s3uri.Host, s3uri.Path), nil
}

func (c StackConfig) EtcdSnapshotsS3Bucket() (string, error) {
	s3uri, err := url.Parse(c.ClusterS3URI())
	if err != nil {
		return "", fmt.Errorf("Error in EtcdSnapshotsS3Bucket : %v", err)
	}
	return s3uri.Host, nil
}

func (c StackConfig) EtcdSnapshotsS3PrefixRef() (string, error) {
	s3uri, err := url.Parse(c.ClusterS3URI())
	if err != nil {
		return "", fmt.Errorf("Error in EtcdSnapshotsS3Prefix : %v", err)
	}
	s3path := fmt.Sprintf(`{ "Fn::Join" : [ "", [ "%s/instances/", { "Fn::Select" : [ "2", { "Fn::Split": [ "/", { "Ref": "AWS::StackId" }]} ]}, "/etcd-snapshots" ]]}`, strings.TrimLeft(s3uri.Path, "/"))
	return s3path, nil
}

func (c *StackConfig) RenderStackTemplateAsBytes() ([]byte, error) {
	return jsontemplate.GetBytes(c.StackTemplateTmplFile, *c, c.PrettyPrint)
}

func (c *StackConfig) RenderStackTemplateAsString() (string, error) {
	bytes, err := c.RenderStackTemplateAsBytes()
	return string(bytes), err
}
