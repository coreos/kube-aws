package cmd

import (
	"fmt"
	"github.com/coreos/kube-aws/core/root"
	"github.com/spf13/cobra"
	"strings"
)

//TODO this is a first step to calculate the stack cost
//this command could scrap aws to print the total cost, rather just showing the link

var (
	cmdCalculator = &cobra.Command{
		Use:          "calculator",
		Short:        "Discovery the monthly cost of your cluster",
		Long:         ``,
		RunE:         runCmdCalculator,
		SilenceUsage: true,
	}

	calculatorOpts = struct {
		awsDebug bool
		s3URI    string
	}{}
)

func init() {
	RootCmd.AddCommand(cmdCalculator)
	cmdCalculator.Flags().BoolVar(&calculatorOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
	cmdCalculator.Flags().StringVar(&calculatorOpts.s3URI, "s3-uri", "", "When your template is bigger than the cloudformation limit of 51200 bytes, upload the template to the specified location in S3. S3 location expressed as s3://<bucket>/path/to/dir")
}

func runCmdCalculator(cmd *cobra.Command, args []string) error {

	if err := validateRequired(flag{"--s3-uri", calculatorOpts.s3URI}); err != nil {
		return err
	}

	opts := root.NewOptions(calculatorOpts.s3URI, false, false)

	cluster, err := root.ClusterFromFile(configPath, opts, calculatorOpts.awsDebug)
	if err != nil {
		return fmt.Errorf("Failed to initialize cluster driver: %v", err)
	}

	if _, err := cluster.ValidateStack(); err != nil {
		return fmt.Errorf("Error validating cluster: %v", err)
	}

	urls, err := cluster.EstimateCost()

	if err != nil {
		return fmt.Errorf("%v", err)
	}

	fmt.Printf("To estimate your monthly cost, open the links below\n%v", strings.Join(urls, "\n"))

	return nil
}
