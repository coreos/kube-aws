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
		awsDebug, skipWait bool
		targets            []string
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
	cmdValidate.Flags().StringSliceVar(
		&validateOpts.targets,
		"targets",
		root.AllOperationTargetsAsStringSlice(),
		"Validate nothing but specified sub-stacks. Specify `all` or any combination of `etcd`, `control-plane`, and node pool names. Defaults to `all`")
}

func runCmdValidate(_ *cobra.Command, _ []string) error {
	opts := root.NewOptions(validateOpts.awsDebug, validateOpts.skipWait)

	cluster, err := root.ClusterFromFile(configPath, opts, validateOpts.awsDebug)
	if err != nil {
		return fmt.Errorf("Failed to initialize cluster driver: %v", err)
	}

	fmt.Printf("Validating UserData and stack template...\n")

	targets := root.OperationTargetsFromStringSlice(validateOpts.targets)

	report, err := cluster.ValidateStack(targets)
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
