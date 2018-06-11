package cmd

import (
	"github.com/kubernetes-incubator/kube-aws/core/root"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/spf13/cobra"
)

var (
	cmdUpdate = &cobra.Command{
		Use:          "update",
		Short:        "DEPRECATED: Update an existing Kubernetes cluster",
		Long:         ``,
		RunE:         runCmdUpdate,
		SilenceUsage: true,
	}
)

func init() {
	RootCmd.AddCommand(cmdUpdate)
	cmdUpdate.Flags().BoolVar(&applyOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
	cmdUpdate.Flags().BoolVar(&applyOpts.prettyPrint, "pretty-print", false, "Pretty print the resulting CloudFormation")
	cmdUpdate.Flags().BoolVar(&applyOpts.skipWait, "skip-wait", false, "Don't wait the resources finish")
	cmdUpdate.Flags().BoolVar(&applyOpts.force, "force", false, "Don't ask for confirmation")
	cmdUpdate.Flags().StringSliceVar(&applyOpts.targets, "targets", root.AllOperationTargetsAsStringSlice(), "Update nothing but specified sub-stacks.  Specify `all` or any combination of `etcd`, `control-plane`, and node pool names. Defaults to `all`")
}

func runCmdUpdate(cobra *cobra.Command, command []string) error {
	logger.Warnf("WARNING! kube-aws 'update' command is deprecated and will be removed in future versions")
	logger.Warnf("Please use 'apply' to update your cluster")

	return runCmdApply(cobra, command)
}
