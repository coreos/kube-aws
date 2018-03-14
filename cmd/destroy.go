package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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
	cmdDestroy.Flags().BoolVar(&destroyOpts.Force, "force", false, "Don't ask for confirmation")
}

func runCmdDestroy(cmd *cobra.Command, args []string) error {
	if !destroyOpts.Force && !destroyConfirmation() {
		fmt.Printf("Operation Cancelled")
		return nil
	}

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

func destroyConfirmation() bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("This operation will destroy the cluster. Are you sure? [y,n]: ")
	text, _ := reader.ReadString('\n')
	text = strings.TrimSuffix(strings.ToLower(text), "\n")

	return text == "y" || text == "yes"
}
