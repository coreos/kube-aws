package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"io/ioutil"

	"github.com/coreos/kube-aws/cluster"
	"github.com/coreos/kube-aws/config"
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
		awsDebug, export, prettyPrint bool
		s3URI                         string
	}{}
)

func init() {
	RootCmd.AddCommand(cmdUp)
	cmdUp.Flags().BoolVar(&upOpts.export, "export", false, "Don't create cluster, instead export cloudformation stack file")
	cmdUp.Flags().BoolVar(&upOpts.prettyPrint, "pretty-print", false, "Pretty print the resulting CloudFormation")
	cmdUp.Flags().BoolVar(&upOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
	cmdUp.Flags().StringVar(&upOpts.s3URI, "s3-uri", "", "When your template is bigger than the cloudformation limit of 51200 bytes, upload the template to the specified location in S3. S3 location expressed as s3://<bucket>/path/to/dir")
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

	conf, err := config.ClusterFromFile(configPath)
	if err != nil {
		return fmt.Errorf("Failed to read cluster config: %v", err)
	}

	opts := stackTemplateOptions(upOpts.s3URI, upOpts.prettyPrint)

	cluster, err := cluster.NewCluster(conf, opts, upOpts.awsDebug)
	if err != nil {
		return fmt.Errorf("Failed to initialize cluster driver : %v", cluster)
	}

	if err := cluster.ValidateUserData(); err != nil {
		return err
	}

	stackTemplate, err := cluster.RenderStackTemplateAsBytes()
	if err != nil {
		return fmt.Errorf("Failed to render stack template: %v", err)
	}

	if err := cluster.Validate(); err != nil {
		return fmt.Errorf("Error validating cluster: %v", err)
	}

	if upOpts.export {
		templatePath := fmt.Sprintf("%s.stack-template.json", conf.ClusterName)
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
