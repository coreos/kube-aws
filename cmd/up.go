package cmd

import (
	"fmt"

	"github.com/coreos/kube-aws/core/root"
	"github.com/spf13/cobra"
)

var (
	cmdUp = &cobra.Command{
		Use:          "up",
		Short:        "Create a new Kubernetes cluster",
		Long:         ``,
		RunE:         runCmdUp,
		SilenceUsage: true,
	}

	upOpts = struct {
		awsDebug, export, prettyPrint, skipWait bool
		s3URI                                   string
	}{}
)

func init() {
	RootCmd.AddCommand(cmdUp)
	cmdUp.Flags().BoolVar(&upOpts.export, "export", false, "Don't create cluster, instead export cloudformation stack file")
	cmdUp.Flags().BoolVar(&upOpts.prettyPrint, "pretty-print", false, "Pretty print the resulting CloudFormation")
	cmdUp.Flags().BoolVar(&upOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
	cmdUp.Flags().StringVar(&upOpts.s3URI, "s3-uri", "", "When your template is bigger than the cloudformation limit of 51200 bytes, upload the template to the specified location in S3. S3 location expressed as s3://<bucket>/path/to/dir")
	cmdUp.Flags().BoolVar(&upOpts.skipWait, "skip-wait", false, "Don't wait for the cluster components be ready")
}

func runCmdUp(cmd *cobra.Command, args []string) error {
	// s3URI is required in order to render stack templates because the URI is parsed, combined and then included in the stack templates as
	// (1) URLs to actual worker/controller cloud-configs in S3 and
	// (2) URLs to nested stack templates referenced from the root stack template
	if err := validateRequired(flag{"--s3-uri", upOpts.s3URI}); err != nil {
		return err
	}

	opts := root.NewOptions(upOpts.s3URI, upOpts.prettyPrint, upOpts.skipWait)

	cluster, err := root.ClusterFromFile(configPath, opts, upOpts.awsDebug)
	if err != nil {
		return fmt.Errorf("Failed to initialize cluster driver: %v", err)
	}

	if _, err := cluster.ValidateStack(); err != nil {
		return fmt.Errorf("Error validating cluster: %v", err)
	}

	if upOpts.export {
		if err := cluster.Export(); err != nil {
			return err
		}
		return nil
	}

	fmt.Printf("Creating AWS resources. Please wait. It may take a few minutes.\n")
	if err := cluster.Create(); err != nil {
		return fmt.Errorf("Error creating cluster: %v", err)
	}

	info, err := cluster.Info()
	if err != nil {
		return fmt.Errorf("Failed fetching cluster info: %v", err)
	}

	successMsg :=
		`Success! Your AWS resources have been created:
%s
The containers that power your cluster are now being downloaded.

You should be able to access the Kubernetes API once the containers finish downloading.
`
	fmt.Printf(successMsg, info.String())

	return nil
}
