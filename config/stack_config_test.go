package config

import (
	"github.com/coreos/kube-aws/model"
	"github.com/coreos/kube-aws/test/helper"
	"testing"
)

func TestRenderStackTemplate(t *testing.T) {
	clusterConfig := newDefaultClusterWithDeps(&dummyEncryptService{})

	clusterConfig.Region = "us-west-1"
	clusterConfig.Subnets = []*model.Subnet{
		{"us-west-1a", "10.0.1.0/16", "", "", "", model.NatGateway{}, false},
		{"us-west-1b", "10.0.2.0/16", "", "", "", model.NatGateway{}, false},
	}

	helper.WithDummyCredentials(func(dir string) {
		var stackTemplateOptions = StackTemplateOptions{
			TLSAssetsDir:          dir,
			ControllerTmplFile:    "templates/cloud-config-controller",
			WorkerTmplFile:        "templates/cloud-config-worker",
			EtcdTmplFile:          "templates/cloud-config-etcd",
			StackTemplateTmplFile: "templates/stack-template.json",
		}

		stackConfig, err := clusterConfig.StackConfig(stackTemplateOptions)
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

	cluster.Region = "us-west-1"
	cluster.Subnets = []*model.Subnet{
		{"us-west-1a", "10.0.1.0/16", "", "", "", model.NatGateway{}, false},
		{"us-west-1b", "10.0.2.0/16", "", "", "", model.NatGateway{}, false},
	}

	helper.WithDummyCredentials(func(dir string) {
		var stackTemplateOptions = StackTemplateOptions{
			TLSAssetsDir:          dir,
			ControllerTmplFile:    "templates/cloud-config-controller",
			WorkerTmplFile:        "templates/cloud-config-worker",
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
