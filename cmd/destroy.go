package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kubernetes-incubator/kube-aws/core/root"
	"github.com/kubernetes-incubator/kube-aws/logger"
)

var (
	cmdDestroy = &cobra.Command{
		Use:          "destroy",
		Short:        "Destroy an existing Kubernetes cluster",
		Long:         ``,
		Run:          runCmdDestroy,
		SilenceUsage: true,
	}
	destroyOpts = root.DestroyOptions{}
)

func init() {
	RootCmd.AddCommand(cmdDestroy)
	cmdDestroy.Flags().BoolVar(&destroyOpts.AwsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
	cmdDestroy.Flags().BoolVar(&destroyOpts.Force, "force", false, "Don't ask for confirmation")
}

func runCmdDestroy(_ *cobra.Command, _ []string) {
	if !destroyOpts.Force && !destroyConfirmation() {
		logger.Info("Operation Cancelled")
		return
	}

	c, err := root.ClusterDestroyerFromFile(configPath, destroyOpts)
	if err != nil {
		logger.Fatalf("Error parsing config: %v", err)
	}

	if err := c.Destroy(); err != nil {
		logger.Fatalf("Failed destroying cluster: %v", err)
	}

	logger.Info("CloudFormation stack is being destroyed. This will take several minutes")
}

func destroyConfirmation() bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("This operation will destroy the cluster. Are you sure? [y,n]: ")
	text, _ := reader.ReadString('\n')
	text = strings.TrimSuffix(strings.ToLower(text), "\n")

	return text == "y" || text == "yes"
}
