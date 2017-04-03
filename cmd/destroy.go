package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kubernetes-incubator/kube-aws/core/root"
)

var (
	cmdDestroy = &cobra.Command{
		Use:          "destroy",
		Short:        "Destroy an existing Kubernetes cluster",
		Long:         ``,
		RunE:         runCmdDestroy,
		SilenceUsage: true,
	}
	destroyOpts = root.DestroyOptions{}
)

func init() {
	RootCmd.AddCommand(cmdDestroy)
	cmdDestroy.Flags().BoolVar(&destroyOpts.AwsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
}

func runCmdDestroy(cmd *cobra.Command, args []string) error {
	c, err := root.ClusterDestroyerFromFile(configPath, destroyOpts)
	if err != nil {
		return fmt.Errorf("Error parsing config: %v", err)
	}

	if err := c.Destroy(); err != nil {
		return fmt.Errorf("Failed destroying cluster: %v", err)
	}

	fmt.Println("CloudFormation stack is being destroyed. This will take several minutes")
	return nil
}
