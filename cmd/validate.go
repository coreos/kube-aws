package cmd

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/core/root"
	"github.com/kubernetes-incubator/kube-aws/logger"
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

	cluster, err := root.LoadClusterFromFile(configPath, opts, validateOpts.awsDebug)
	if err != nil {
		return fmt.Errorf("failed to initialize cluster driver: %v", err)
	}

	logger.Info("Validating UserData and stack template...\n")

	targets := root.OperationTargetsFromStringSlice(validateOpts.targets)

	report, err := cluster.ValidateStack(targets)
	if report != "" {
		logger.Infof("Validation Report: %s\n", report)
	}
	if err != nil {
		return err
	}

	logger.Info("stack template is valid.\n\n")
	logger.Info("Validation OK!")
	return nil
}
