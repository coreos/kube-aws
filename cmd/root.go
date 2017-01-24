package cmd

import (
	"github.com/coreos/kube-aws/config"
	"github.com/spf13/cobra"
)

var (
	RootCmd = &cobra.Command{
		Use:   "kube-aws",
		Short: "Manage Kubernetes clusters on AWS",
		Long:  ``,
	}

	configPath = "cluster.yaml"
)

func stackTemplateOptions(s3URI string, prettyPrint bool) config.StackTemplateOptions {
	return config.StackTemplateOptions{
		TLSAssetsDir:          "credentials",
		ControllerTmplFile:    "userdata/cloud-config-controller",
		WorkerTmplFile:        "userdata/cloud-config-worker",
		EtcdTmplFile:          "userdata/cloud-config-etcd",
		StackTemplateTmplFile: "stack-template.json",
		S3URI:       s3URI,
		PrettyPrint: prettyPrint,
	}
}
