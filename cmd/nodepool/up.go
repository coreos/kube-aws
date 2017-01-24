package nodepool

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/coreos/kube-aws/nodepool/cluster"
	"github.com/coreos/kube-aws/nodepool/config"
	"github.com/spf13/cobra"
	"io/ioutil"
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
		awsDebug, export, prettyPrint bool
		s3URI                         string
	}{}
)

func init() {
	NodePoolCmd.AddCommand(cmdUp)
	cmdUp.Flags().BoolVar(&upOpts.export, "export", false, "Don't create cluster, instead export cloudformation stack file")
	cmdUp.Flags().BoolVar(&upOpts.prettyPrint, "pretty-print", false, "Pretty print the resulting CloudFormation")
	cmdUp.Flags().BoolVar(&upOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
	cmdUp.Flags().StringVar(&upOpts.s3URI, "s3-uri", "", "When your template is bigger than the cloudformation limit of 51200 bytes, upload the template to the specified location in S3")
}

func runCmdUp(cmd *cobra.Command, args []string) error {
	// Up flags.
	required := []struct {
		name, val string
	}{
		{"--s3-uri", upOpts.s3URI},
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

	conf, err := config.ClusterFromFile(nodePoolClusterConfigFilePath())
	if err != nil {
		return fmt.Errorf("Failed to read cluster config: %v", err)
	}

	opts := stackTemplateOptions(upOpts.s3URI, upOpts.prettyPrint)

	cluster, err := cluster.NewCluster(conf, opts, upOpts.awsDebug)
	if err != nil {
		return fmt.Errorf("Failed to initialize cluster driver : %v", err)
	}

	if err := cluster.ValidateUserData(); err != nil {
		return fmt.Errorf("Failed to validate user data: %v", err)
	}

	stackTemplate, err := cluster.RenderStackTemplateAsBytes()
	if err != nil {
		return fmt.Errorf("Failed to render stack template: %v", err)
	}

	if upOpts.export {
		templatePath := nodePoolExportedStackTemplatePath()
		fmt.Printf("Exporting %s\n", templatePath)
		if err := ioutil.WriteFile(templatePath, stackTemplate, 0600); err != nil {
			return fmt.Errorf("Error writing %s : %v", templatePath, err)
		}
		if conf.KMSKeyARN == "" {
			fmt.Printf("BEWARE: %s contains your TLS secrets!\n", templatePath)
		}
		return nil
	}

	fmt.Printf("Creating AWS resources. This should take around 5 minutes.\n")
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
