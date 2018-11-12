package cmd

import (
	"fmt"

	"github.com/kubernetes-incubator/kube-aws/core/root"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/spf13/cobra"
	"strings"
)

var (
	cmdDiff = &cobra.Command{
		Use:          "diff",
		Short:        "Compare the current and the desired states of the cluster",
		Long:         ``,
		RunE:         runCmdDiff,
		SilenceUsage: true,
	}

	diffOpts = struct {
		awsDebug, prettyPrint, skipWait, export bool
		context                                 int
		targets                                 []string
	}{}
)

type ExitError struct {
	msg  string
	Code int
}

func (e *ExitError) Error() string {
	return e.msg
}

func init() {
	RootCmd.AddCommand(cmdDiff)
	cmdDiff.Flags().BoolVar(&diffOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
	cmdDiff.Flags().StringSliceVar(&diffOpts.targets, "targets", root.AllOperationTargetsAsStringSlice(), "Diff nothing but specified sub-stacks.  Specify `all` or any combination of `etcd`, `control-plane`, and node pool names. Defaults to `all`")
	cmdDiff.Flags().IntVarP(&diffOpts.context, "context", "C", -1, "output NUM lines of context around changes")
}

func runCmdDiff(c *cobra.Command, _ []string) error {
	opts := root.NewOptions(diffOpts.prettyPrint, diffOpts.skipWait)

	cluster, err := root.LoadClusterFromFile(configPath, opts, diffOpts.awsDebug)
	if err != nil {
		return fmt.Errorf("failed to read cluster config: %v", err)
	}

	targets := root.OperationTargetsFromStringSlice(diffOpts.targets)

	if _, err := cluster.ValidateStack(targets); err != nil {
		return err
	}

	diffs, err := cluster.Diff(targets, diffOpts.context)
	if err != nil {
		return fmt.Errorf("error comparing cluster states: %v", err)
	}

	for _, diff := range diffs {
		logger.Infof("Detected changes in: %s\n%s", diff.Target, diff.String())
	}

	names := make([]string, len(diffs))
	for i := range diffs {
		names[i] = diffs[i].Target
	}

	if len(diffs) > 0 {
		c.SilenceErrors = true
		return &ExitError{fmt.Sprintf("Detected changes in: %s", strings.Join(names, ", ")), 2}
	}

	return nil
}
