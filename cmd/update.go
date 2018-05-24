package cmd

import (
	"fmt"

	"bufio"
	"github.com/kubernetes-incubator/kube-aws/core/root"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

var (
	cmdUpdate = &cobra.Command{
		Use:          "update",
		Short:        "Update an existing Kubernetes cluster",
		Long:         ``,
		Run:          runCmdUpdate,
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

func runCmdUpdate(_ *cobra.Command, _ []string) {
	if !updateOpts.force && !updateConfirmation() {
		logger.Info("Operation cancelled")
		return
	}

	opts := root.NewOptions(updateOpts.prettyPrint, updateOpts.skipWait)

	cluster, err := root.ClusterFromFile(configPath, opts, updateOpts.awsDebug)
	if err != nil {
		logger.Fatalf("Failed to read cluster config: %v", err)
	}

	targets := root.OperationTargetsFromStringSlice(updateOpts.targets)

	if _, err := cluster.ValidateStack(targets); err != nil {
		logger.Fatal(err)
	}

	report, err := cluster.Update(targets)
	if err != nil {
		logger.Fatalf("Error updating cluster: %v", err)
	}
	if report != "" {
		logger.Infof("Update stack: %s\n", report)
	}

	info, err := cluster.Info()
	if err != nil {
		logger.Fatalf("Failed fetching cluster info: %v", err)
	}

	successMsg :=
		`Success! Your AWS resources are being updated:
%s
`
	logger.Infof(successMsg, info)
}

func updateConfirmation() bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("This operation will update the cluster. Are you sure? [y,n]: ")
	text, _ := reader.ReadString('\n')
	text = strings.TrimSuffix(strings.ToLower(text), "\n")

	return text == "y" || text == "yes"
}
