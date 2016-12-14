package nodepool

import (
	"fmt"

	"github.com/coreos/kube-aws/nodepool/cluster"
	"github.com/coreos/kube-aws/nodepool/config"
	"github.com/spf13/cobra"
)

var (
	cmdUpdate = &cobra.Command{
		Use:          "update",
		Short:        "Update an existing node pool",
		Long:         ``,
		RunE:         runCmdUpdate,
		SilenceUsage: true,
	}

	updateOpts = struct {
		awsDebug, prettyPrint bool
		s3URI                 string
	}{}
)

func init() {
	NodePoolCmd.AddCommand(cmdUpdate)
	cmdUpdate.Flags().BoolVar(&updateOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
	cmdUpdate.Flags().BoolVar(&updateOpts.prettyPrint, "pretty-print", false, "Pretty print the resulting CloudFormation")
	cmdUpdate.Flags().StringVar(&updateOpts.s3URI, "s3-uri", "", "When your template is bigger than the cloudformation limit of 51200 bytes, upload the template to the specified location in S3. S3 location expressed as s3://<bucket>/path/to/dir")
}

func runCmdUpdate(cmd *cobra.Command, args []string) error {
	conf, err := config.ClusterFromFile(nodePoolClusterConfigFilePath())
	if err != nil {
		return fmt.Errorf("Failed to read node pool config: %v", err)
	}

	if err := conf.ValidateUserData(stackTemplateOptions()); err != nil {
		return fmt.Errorf("Failed to validate user data: %v", err)
	}

	data, err := conf.RenderStackTemplate(stackTemplateOptions(), updateOpts.prettyPrint)
	if err != nil {
		return fmt.Errorf("Failed to render stack template: %v", err)
	}

	cluster := cluster.New(conf, updateOpts.awsDebug)

	report, err := cluster.Update(string(data), updateOpts.s3URI)
	if err != nil {
		return fmt.Errorf("Error updating node pool: %v", err)
	}
	if report != "" {
		fmt.Printf("Update stack: %s\n", report)
	}

	info, err := cluster.Info()
	if err != nil {
		return fmt.Errorf("Failed fetching node pool info: %v", err)
	}

	successMsg :=
		`Success! Your AWS resources are being updated:
%s
`
	fmt.Printf(successMsg, info.String())

	return nil
}
