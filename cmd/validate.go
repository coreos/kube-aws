package cmd

import (
	"fmt"
	"os"

	"github.com/kubernetes-incubator/kube-aws/core/root"
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
}

func runCmdValidate(cmd *cobra.Command, args []string) error {
	opts := root.NewOptions(validateOpts.awsDebug, validateOpts.skipWait)

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
	fmt.Println("Validation OK!")

	return nil
}
