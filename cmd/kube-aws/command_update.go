package main

import (
	"fmt"

	"github.com/coreos/kube-aws/cluster"
	"github.com/coreos/kube-aws/config"
	"github.com/spf13/cobra"
)

var (
	cmdUpdate = &cobra.Command{
		Use:          "update",
		Short:        "Update an existing Kubernetes cluster",
		Long:         ``,
		RunE:         runCmdUpdate,
		SilenceUsage: true,
	}

	updateOpts = struct {
		awsDebug bool
		s3URI    string
	}{}
)

func init() {
	cmdRoot.AddCommand(cmdUpdate)
	cmdUpdate.Flags().BoolVar(&updateOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
	cmdUpdate.Flags().StringVar(&updateOpts.s3URI, "s3-uri", "", "When your template is bigger than the cloudformation limit of 51200 bytes, upload the template to the specified location in S3")
}

func runCmdUpdate(cmd *cobra.Command, args []string) error {
	conf, err := config.ClusterFromFile(configPath)
	if err != nil {
		return fmt.Errorf("Failed to read cluster config: %v", err)
	}

	if err := conf.ValidateUserData(stackTemplateOptions); err != nil {
		return err
	}

	data, err := conf.RenderStackTemplate(stackTemplateOptions)
	if err != nil {
		return fmt.Errorf("Failed to render stack template: %v", err)
	}

	cluster := cluster.New(conf, updateOpts.awsDebug)

	report, err := cluster.Update(string(data), updateOpts.s3URI)
	if err != nil {
		return fmt.Errorf("Error updating cluster: %v", err)
	}
	if report != "" {
		fmt.Printf("Update stack: %s\n", report)
	}

	info, err := cluster.Info()
	if err != nil {
		return fmt.Errorf("Failed fetching cluster info: %v", err)
	}

	successMsg :=
		`Success! Your AWS resources are being updated:
`
	fmt.Printf(successMsg, info.String())

	return nil
}
