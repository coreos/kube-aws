package cmd

import (
	"fmt"

	"github.com/coreos/kube-aws/cluster"
	"github.com/coreos/kube-aws/config"
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

func runCmdStatus(cmd *cobra.Command, args []string) error {
	conf, err := config.ClusterFromFile(configPath)
	if err != nil {
		return fmt.Errorf("Failed to read cluster config: %v", err)
	}

	info, err := cluster.NewClusterRef(conf, false).Info()
	if err != nil {
		return fmt.Errorf("Failed fetching cluster info: %v", err)
	}

	fmt.Print(info.String())
	return nil
}
