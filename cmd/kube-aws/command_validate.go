package main

import (
	"fmt"
	"os"

	"github.com/coreos/kube-aws/cluster"
	"github.com/coreos/kube-aws/config"
	"github.com/spf13/cobra"
)

var (
	cmdValidate = &cobra.Command{
		Use:          "validate",
		Short:        "Validate cluster assets",
		Long:         ``,
		RunE:         runCmdValidate,
		SilenceUsage: true,
	}

	validateOpts = struct {
		awsDebug bool
		s3URI    string
	}{}
)

func init() {
	cmdRoot.AddCommand(cmdValidate)
	cmdValidate.Flags().BoolVar(
		&validateOpts.awsDebug,
		"aws-debug",
		false,
		"Log debug information from aws-sdk-go library",
	)
	cmdValidate.Flags().StringVar(
		&validateOpts.s3URI,
		"s3-uri",
		"",
		"When your template is bigger than the cloudformation limit of 51200 bytes, upload the template to the specified location in S3",
	)
}

func runCmdValidate(cmd *cobra.Command, args []string) error {
	cfg, err := config.ClusterFromFile(configPath)
	if err != nil {
		return fmt.Errorf("Unable to load cluster config: %v", err)
	}

	fmt.Printf("Validating UserData...\n")
	if err := cfg.ValidateUserData(stackTemplateOptions); err != nil {
		return err
	}
	fmt.Printf("UserData is valid.\n\n")

	fmt.Printf("Validating stack template...\n")
	data, err := cfg.RenderStackTemplate(stackTemplateOptions)
	if err != nil {
		return fmt.Errorf("Failed to render stack template: %v", err)
	}

	cluster := cluster.New(cfg, validateOpts.awsDebug)
	report, err := cluster.ValidateStack(string(data), validateOpts.s3URI)
	if report != "" {
		fmt.Fprintf(os.Stderr, "Validation Report: %s\n", report)
	}

	if err != nil {
		return err
	}
	fmt.Printf("stack template is valid.\n\n")

	fmt.Printf("Validation OK!\n")
	return nil
}
