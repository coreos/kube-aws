package cmd

import (
	"github.com/kubernetes-incubator/kube-aws/core/root"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/spf13/cobra"
)

var (
	cmdStatus = &cobra.Command{
		Use:          "status",
		Short:        "Describe an existing Kubernetes cluster",
		Long:         ``,
		Run:          runCmdStatus,
		SilenceUsage: true,
	}
)

func init() {
	RootCmd.AddCommand(cmdStatus)
}

func runCmdStatus(_ *cobra.Command, _ []string) {
	describer, err := root.ClusterDescriberFromFile(configPath)
	if err != nil {
		logger.Fatalf("Failed to read cluster config: %v", err)
	}

	info, err := describer.Info()
	if err != nil {
		logger.Fatalf("Failed fetching cluster info: %v", err)
	}

	logger.Info(info)
}
