package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/coreos/kube-aws/cluster"
	"github.com/coreos/kube-aws/config"
)

var (
	cmdDestroy = &cobra.Command{
		Use:          "destroy",
		Short:        "Destroy an existing Kubernetes cluster",
		Long:         ``,
		RunE:         runCmdDestroy,
		SilenceUsage: true,
	}
	destroyOpts = struct {
		awsDebug bool
	}{}
)

func init() {
	RootCmd.AddCommand(cmdDestroy)
	cmdDestroy.Flags().BoolVar(&destroyOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
}

func runCmdDestroy(cmd *cobra.Command, args []string) error {
	cfg, err := config.ClusterFromFile(configPath)
	if err != nil {
		return fmt.Errorf("Error parsing config: %v", err)
	}

	c := cluster.NewClusterRef(cfg, destroyOpts.awsDebug)
	if err := c.Destroy(); err != nil {
		return fmt.Errorf("Failed destroying cluster: %v", err)
	}

	fmt.Println("CloudFormation stack is being destroyed. This will take several minutes")
	return nil
}
