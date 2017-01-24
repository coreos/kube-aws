package nodepool

import (
	"fmt"

	"github.com/coreos/kube-aws/nodepool/cluster"
	"github.com/coreos/kube-aws/nodepool/config"
	"github.com/spf13/cobra"
	"os"
)

var (
	cmdValidate = &cobra.Command{
		Use:          "validate",
		Short:        "Validate node pool assets",
		Long:         ``,
		RunE:         runCmdValidate,
		SilenceUsage: true,
	}

	validateOpts = struct {
		awsDebug bool
		s3URI    string
	}{}
)

func init() {
	NodePoolCmd.AddCommand(cmdValidate)
	cmdValidate.Flags().BoolVar(&validateOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
	cmdValidate.Flags().StringVar(&validateOpts.s3URI, "s3-uri", "", "When your template is bigger than the cloudformation limit of 51200 bytes, upload the template to the specified location in S3. S3 location expressed as s3://<bucket>/path/to/dir")
}

func runCmdValidate(cmd *cobra.Command, args []string) error {
	conf, err := config.ClusterFromFile(nodePoolClusterConfigFilePath())
	if err != nil {
		return fmt.Errorf("Failed to read node pool config: %v", err)
	}

	opts := stackTemplateOptions(validateOpts.s3URI, false)
	cluster, err := cluster.NewCluster(conf, opts, validateOpts.awsDebug)
	if err != nil {
		return fmt.Errorf("Failed to initialize cluster driver : %v ", cluster)
	}

	report, err := cluster.ValidateStack()
	if err := cluster.ValidateUserData(); err != nil {
		return fmt.Errorf("Failed to validate user data: %v", err)
	}

	if report != "" {
		fmt.Fprintf(os.Stderr, "Validation Report: %s\n", report)
	}

	if err != nil {
		return err
	}
	fmt.Printf("stack template is valid.\n\n")

	fmt.Printf("Validation OK!\n")
	return nil
}
