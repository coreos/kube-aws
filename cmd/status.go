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
)

func init() {
	RootCmd.AddCommand(cmdStatus)
}

func runCmdStatus(_ *cobra.Command, _ []string) error {
	describer, err := root.ClusterDescriberFromFile(configPath)
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
