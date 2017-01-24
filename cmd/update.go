package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/coreos/kube-aws/cluster"
	"github.com/coreos/kube-aws/config"
	"github.com/spf13/cobra"
)

var (
	cmdUpdate = &cobra.Command{
		Use:          "update",
		Short:        "Update an existing Kubernetes cluster",
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
	RootCmd.AddCommand(cmdUpdate)
	cmdUpdate.Flags().BoolVar(&updateOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
	cmdUpdate.Flags().BoolVar(&updateOpts.prettyPrint, "pretty-print", false, "Pretty print the resulting CloudFormation")
	cmdUpdate.Flags().StringVar(&updateOpts.s3URI, "s3-uri", "", "When your template is bigger than the cloudformation limit of 51200 bytes, upload the template to the specified location in S3. S3 location expressed as s3://<bucket>/path/to/dir")
}

func runCmdUpdate(cmd *cobra.Command, args []string) error {
	// Up flags.
	required := []struct {
		name, val string
	}{
		{"--s3-uri", updateOpts.s3URI},
	}
	var missing []string
	for _, req := range required {
		if req.val == "" {
			missing = append(missing, strconv.Quote(req.name))
		}
	}
	if len(missing) != 0 {
		return fmt.Errorf("Missing required flag(s): %s", strings.Join(missing, ", "))
	}

	confCluster, err := config.ClusterFromFile(configPath)
	if err != nil {
		return fmt.Errorf("Failed to read cluster config: %v", err)
	}

	opts := stackTemplateOptions(updateOpts.s3URI, updateOpts.prettyPrint)

	cluster, err := cluster.NewCluster(confCluster, opts, upOpts.awsDebug)
	if err != nil {
		return fmt.Errorf("Failed to initialize cluster driver : %v", cluster)
	}

	if err := cluster.ValidateUserData(); err != nil {
		return err
	}

	report, err := cluster.Update()
	if err != nil {
		return fmt.Errorf("Error updating cluster: %v", err)
	}
	if report != "" {
		fmt.Printf("Update stack: %s\n", report)
	}

	info, err := cluster.Info()
	if err != nil {
		return fmt.Errorf("Failed fetching cluster info: %v", err)
	}

	successMsg :=
		`Success! Your AWS resources are being updated:
%s
`
	fmt.Printf(successMsg, info.String())

	return nil
}
