package cmd

import (
	"fmt"

	"github.com/kubernetes-incubator/kube-aws/core/root"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/spf13/cobra"
)

var (
	cmdStatus = &cobra.Command{
		Use:          "status",
		Short:        "Describe an existing Kubernetes cluster",
		Long:         ``,
		RunE:         runCmdStatus,
		SilenceUsage: true,
	}

	statusOpts = struct {
		profile string
	}{}
)

func init() {
	RootCmd.AddCommand(cmdStatus)
	cmdStatus.Flags().StringVar(&statusOpts.profile, "profile", "", "The AWS profile to use from credentials file")
}

func runCmdStatus(_ *cobra.Command, _ []string) error {
	opts := root.NewOptions(false, false, statusOpts.profile)
	describer, err := root.ClusterDescriberFromFile(configPath, opts)
	if err != nil {
		return fmt.Errorf("failed to read cluster config: %v", err)
	}

	info, err := describer.Info()
	if err != nil {
		return fmt.Errorf("failed fetching cluster info: %v", err)
	}

	logger.Info(info)
	return nil
}
