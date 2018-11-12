package cmd

import (
	"fmt"
	"strings"

	"github.com/kubernetes-incubator/kube-aws/core/root"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/spf13/cobra"
)

//TODO this is a first step to calculate the stack cost
//this command could scrap aws to print the total cost, rather just showing the link

var (
	cmdCalculator = &cobra.Command{
		Use:          "calculator",
		Short:        "Discover the monthly cost of your cluster",
		Long:         ``,
		RunE:         runCmdCalculator,
		SilenceUsage: true,
	}

	calculatorOpts = struct {
		awsDebug bool
	}{}
)

func init() {
	RootCmd.AddCommand(cmdCalculator)
	cmdCalculator.Flags().BoolVar(&calculatorOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
}

func runCmdCalculator(_ *cobra.Command, _ []string) error {

	opts := root.NewOptions(false, false)

	cluster, err := root.LoadClusterFromFile(configPath, opts, calculatorOpts.awsDebug)
	if err != nil {
		return fmt.Errorf("failed to initialize cluster driver: %v", err)
	}

	if _, err := cluster.ValidateStack(); err != nil {
		return fmt.Errorf("error validating cluster: %v", err)
	}

	urls, err := cluster.EstimateCost()

	if err != nil {
		return fmt.Errorf("%v", err)
	}

	logger.Heading("To estimate your monthly cost, open the links below")
	logger.Infof("%v", strings.Join(urls, "\n"))
	return nil
}
