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
	cmdApply = &cobra.Command{
		Use:          "apply",
		Short:        "Create or Update your cluster",
		Long:         ``,
		RunE:         runCmdApply,
		SilenceUsage: true,
	}

	applyOpts = struct {
		awsDebug, prettyPrint, skipWait, export bool
		force                                   bool
		targets                                 []string
	}{}
)

func init() {
	RootCmd.AddCommand(cmdApply)
	cmdApply.Flags().BoolVar(&applyOpts.export, "export", false, "Don't create cluster, instead export cloudformation stack file")
	cmdApply.Flags().BoolVar(&applyOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
	cmdApply.Flags().BoolVar(&applyOpts.prettyPrint, "pretty-print", false, "Pretty print the resulting CloudFormation")
	cmdApply.Flags().BoolVar(&applyOpts.skipWait, "skip-wait", false, "Don't wait the resources finish")
	cmdApply.Flags().BoolVar(&applyOpts.force, "force", false, "Don't ask for confirmation")
	cmdApply.Flags().StringSliceVar(&applyOpts.targets, "targets", root.AllOperationTargetsAsStringSlice(), "Update nothing but specified sub-stacks.  Specify `all` or any combination of `etcd`, `control-plane`, and node pool names. Defaults to `all`")
}

func runCmdApply(_ *cobra.Command, _ []string) error {
	if !applyOpts.force && !applyConfirmation() {
		logger.Info("Operation cancelled")
		return nil
	}

	opts := root.NewOptions(applyOpts.prettyPrint, applyOpts.skipWait)

	cluster, err := root.LoadClusterFromFile(configPath, opts, applyOpts.awsDebug)
	if err != nil {
		return fmt.Errorf("failed to read cluster config: %v", err)
	}

	targets := root.OperationTargetsFromStringSlice(applyOpts.targets)

	if _, err := cluster.ValidateStack(targets); err != nil {
		return err
	}

	if applyOpts.export {
		if err := cluster.Export(); err != nil {
			return err
		}
		return nil
	}

	err = cluster.Apply(targets)
	if err != nil {
		return fmt.Errorf("error updating cluster: %v", err)
	}

	info, err := cluster.Info()
	if err != nil {
		return fmt.Errorf("failed fetching cluster info: %v", err)
	}

	successMsg :=
		`Success! Your AWS resources are being provisioned:
%s
`
	logger.Infof(successMsg, info)
	return nil
}

func applyConfirmation() bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("This operation will create/update the cluster. Are you sure? [y,n]: ")
	text, _ := reader.ReadString('\n')
	text = strings.TrimSuffix(strings.ToLower(text), "\n")

	return text == "y" || text == "yes"
}
