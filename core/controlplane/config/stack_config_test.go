package config

import (
	"github.com/kubernetes-incubator/kube-aws/model"
	"github.com/kubernetes-incubator/kube-aws/test/helper"
	"testing"
)

func TestRenderStackTemplate(t *testing.T) {
	cluster := newDefaultClusterWithDeps(&dummyEncryptService{})

	cluster.Region = model.RegionForName("us-west-1")
	cluster.Subnets = []model.Subnet{
		model.NewPublicSubnet("us-west-1a", "10.0.1.0/24"),
		model.NewPublicSubnet("us-west-1b", "10.0.2.0/24"),
	}
	cluster.ExternalDNSName = "foo.example.com"
	cluster.KeyName = "mykey"
	cluster.KMSKeyARN = "mykmskey"
	if err := cluster.Load(); err != nil {
		t.Errorf("load failed: %v\n%+v", err, cluster.Subnets)
		t.FailNow()
	}

	helper.WithDummyCredentials(func(dir string) {
		var stackTemplateOptions = StackTemplateOptions{
			AssetsDir:             dir,
			ControllerTmplFile:    "templates/cloud-config-controller",
			EtcdTmplFile:          "templates/cloud-config-etcd",
			StackTemplateTmplFile: "templates/stack-template.json",
		}

		stackConfig, err := cluster.StackConfig(stackTemplateOptions)
		if err != nil {
			t.Errorf("failed to generate stack config : %v", err)
		}

		compressed, err := stackConfig.Compress()
		if err != nil {
			t.Errorf("failed to compress : %v", err)
		}

		if _, err := compressed.RenderStackTemplateAsString(); err != nil {
			t.Errorf("failed to render stack template: %v", err)
		}
	})
}

func TestValidateUserData(t *testing.T) {
	cluster := newDefaultClusterWithDeps(&dummyEncryptService{})

	cluster.Region = model.RegionForName("us-west-1")
	cluster.Subnets = []model.Subnet{
		model.NewPublicSubnet("us-west-1a", "10.0.1.0/24"),
		model.NewPublicSubnet("us-west-1b", "10.0.2.0/24"),
	}
	cluster.ExternalDNSName = "foo.example.com"
	cluster.KeyName = "mykey"
	cluster.KMSKeyARN = "mykmskey"
	if err := cluster.Load(); err != nil {
		t.Errorf("load failed: %v", err)
		t.FailNow()
	}

	helper.WithDummyCredentials(func(dir string) {
		var stackTemplateOptions = StackTemplateOptions{
			AssetsDir:             dir,
			ControllerTmplFile:    "templates/cloud-config-controller",
			EtcdTmplFile:          "templates/cloud-config-etcd",
			StackTemplateTmplFile: "templates/stack-template.json",
		}

		stackConfig, err := cluster.StackConfig(stackTemplateOptions)
		if err != nil {
			t.Errorf("failed to generate stack config : %v", err)
		}

		if err := stackConfig.ValidateUserData(); err != nil {
			t.Errorf("failed to validate user data: %v", err)
		}
	})
}
