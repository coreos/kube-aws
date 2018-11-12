package cmd

import (
	"fmt"

	"github.com/kubernetes-incubator/kube-aws/core/root"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/spf13/cobra"
)

var (
	cmdUp = &cobra.Command{
		Use:          "up",
		Short:        "DEPRECATED: Create a new Kubernetes cluster",
		Long:         ``,
		RunE:         runCmdUp,
		SilenceUsage: true,
	}

	upOpts = struct {
		awsDebug, export, prettyPrint, skipWait bool
	}{}
)

func init() {
	RootCmd.AddCommand(cmdUp)
	cmdUp.Flags().BoolVar(&upOpts.export, "export", false, "Don't create cluster, instead export cloudformation stack file")
	cmdUp.Flags().BoolVar(&upOpts.prettyPrint, "pretty-print", false, "Pretty print the resulting CloudFormation")
	cmdUp.Flags().BoolVar(&upOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
	cmdUp.Flags().BoolVar(&upOpts.skipWait, "skip-wait", false, "Don't wait for the cluster components be ready")
}

func runCmdUp(_ *cobra.Command, _ []string) error {
	logger.Warnf("WARNING! kube-aws 'up' command is deprecated and will be removed in future versions")
	logger.Warnf("Please use 'apply' to create your cluster")

	opts := root.NewOptions(upOpts.prettyPrint, upOpts.skipWait)

	cluster, err := root.LoadClusterFromFile(configPath, opts, upOpts.awsDebug)
	if err != nil {
		return fmt.Errorf("failed to initialize cluster driver: %v", err)
	}

	if _, err := cluster.ValidateStack(); err != nil {
		return fmt.Errorf("error validating cluster: %v", err)
	}

	if upOpts.export {
		if err := cluster.Export(); err != nil {
			return err
		}
		return nil
	}

	logger.Info("Creating AWS resources. Please wait. It may take a few minutes.")
	if err := cluster.LegacyCreate(); err != nil {
		return fmt.Errorf("error creating cluster: %v", err)
	}

	info, err := cluster.Info()
	if err != nil {
		return fmt.Errorf("failed fetching cluster info: %v", err)
	}

	successMsg :=
		`Success! Your AWS resources have been created:
%s
The containers that power your cluster are now being downloaded.
You should be able to access the Kubernetes API once the containers finish downloading.
`
	logger.Infof(successMsg, info)
	return nil
}
