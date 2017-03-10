package cmd

import (
	"fmt"
	"os"

	"github.com/coreos/kube-aws/core/root"
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
		skipWait bool
		s3URI    string
	}{}
)

func init() {
	RootCmd.AddCommand(cmdValidate)
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
		"When your template is bigger than the cloudformation limit of 51200 bytes, upload the template to the specified location in S3. S3 location expressed as s3://<bucket>/path/to/dir",
	)
}

func runCmdValidate(cmd *cobra.Command, args []string) error {
	opts := root.NewOptions(validateOpts.s3URI, validateOpts.awsDebug, validateOpts.skipWait)

	cluster, err := root.ClusterFromFile(configPath, opts, validateOpts.awsDebug)
	if err != nil {
		return fmt.Errorf("Failed to initialize cluster driver: %v", err)
	}

	fmt.Printf("Validating UserData and stack template...\n")
	report, err := cluster.ValidateStack()
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
