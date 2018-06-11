package cmd

import (
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
)

func init() {
	RootCmd.AddCommand(cmdUp)
	cmdUp.Flags().BoolVar(&applyOpts.export, "export", false, "Don't create cluster, instead export cloudformation stack file")
	cmdUp.Flags().BoolVar(&applyOpts.prettyPrint, "pretty-print", false, "Pretty print the resulting CloudFormation")
	cmdUp.Flags().BoolVar(&applyOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
	cmdUp.Flags().BoolVar(&applyOpts.skipWait, "skip-wait", false, "Don't wait for the cluster components be ready")
}

func runCmdUp(cobra *cobra.Command, command []string) error {
	logger.Warnf("WARNING! kube-aws 'up' command is deprecated and will be removed in future versions")
	logger.Warnf("Please use 'apply' to create your cluster")

	return runCmdApply(cobra, command)
}
