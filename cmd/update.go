package cmd

import (
	"fmt"

	"bufio"
	"os"
	"strings"

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

	updateOpts = struct {
		awsDebug, prettyPrint, skipWait bool
		force                           bool
		targets                         []string
	}{}
)

func init() {
	RootCmd.AddCommand(cmdUpdate)
	cmdUpdate.Flags().BoolVar(&updateOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
	cmdUpdate.Flags().BoolVar(&updateOpts.prettyPrint, "pretty-print", false, "Pretty print the resulting CloudFormation")
	cmdUpdate.Flags().BoolVar(&updateOpts.skipWait, "skip-wait", false, "Don't wait the resources finish")
	cmdUpdate.Flags().BoolVar(&updateOpts.force, "force", false, "Don't ask for confirmation")
	cmdUpdate.Flags().StringSliceVar(&updateOpts.targets, "targets", root.AllOperationTargetsAsStringSlice(), "Update nothing but specified sub-stacks.  Specify `all` or any combination of `etcd`, `control-plane`, and node pool names. Defaults to `all`")
}

func runCmdUpdate(_ *cobra.Command, _ []string) error {
	logger.Warnf("WARNING! kube-aws 'update' command is deprecated and will be removed in future versions")
	logger.Warnf("Please use 'apply' to update your cluster")

	if !updateOpts.force && !updateConfirmation() {
		logger.Info("Operation cancelled")
		return nil
	}

	opts := root.NewOptions(updateOpts.prettyPrint, updateOpts.skipWait)

	cluster, err := root.LoadClusterFromFile(configPath, opts, updateOpts.awsDebug)
	if err != nil {
		return fmt.Errorf("failed to read cluster config: %v", err)
	}

	targets := root.OperationTargetsFromStringSlice(updateOpts.targets)

	if _, err := cluster.ValidateStack(targets); err != nil {
		return err
	}

	report, err := cluster.LegacyUpdate(targets)
	if err != nil {
		return fmt.Errorf("error updating cluster: %v", err)
	}
	if report != "" {
		logger.Infof("Update stack: %s\n", report)
	}

	info, err := cluster.Info()
	if err != nil {
		return fmt.Errorf("failed fetching cluster info: %v", err)
	}

	successMsg :=
		`Success! Your AWS resources are being updated:
%s
`
	logger.Infof(successMsg, info)
	return nil
}

func updateConfirmation() bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("This operation will update the cluster. Are you sure? [y,n]: ")
	text, _ := reader.ReadString('\n')
	text = strings.TrimSuffix(strings.ToLower(text), "\n")

	return text == "y" || text == "yes"
}
