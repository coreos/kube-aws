package main

import (
	"fmt"

	"github.com/coreos/kube-aws/cluster"
	"github.com/coreos/kube-aws/config"
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
		awsDebug, export bool
		s3URI            string
	}{}
)

func init() {
	cmdRoot.AddCommand(cmdUp)
	cmdUp.Flags().BoolVar(&upOpts.export, "export", false, "Don't create cluster, instead export cloudformation stack file")
	cmdUp.Flags().BoolVar(&upOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
	cmdUp.Flags().StringVar(&upOpts.s3URI, "s3-uri", "", "When your template is bigger than the cloudformation limit of 51200 bytes, upload the template to the specified location in S3")
}

func runCmdUp(cmd *cobra.Command, args []string) error {
	conf, err := config.ClusterFromFile(configPath)
	if err != nil {
		return fmt.Errorf("Failed to read cluster config: %v", err)
	}

	if err := conf.ValidateUserData(stackTemplateOptions); err != nil {
		return err
	}

	data, err := conf.RenderStackTemplate(stackTemplateOptions)
	if err != nil {
		return fmt.Errorf("Failed to render stack template: %v", err)
	}

	if upOpts.export {
		templatePath := fmt.Sprintf("%s.stack-template.json", conf.ClusterName)
		fmt.Printf("Exporting %s\n", templatePath)
		if err := ioutil.WriteFile(templatePath, data, 0600); err != nil {
			return fmt.Errorf("Error writing %s : %v", templatePath, err)
		}
		if conf.KMSKeyARN == "" {
			fmt.Printf("BEWARE: %s contains your TLS secrets!\n", templatePath)
		}
		return nil
	}

	cluster := cluster.New(conf, upOpts.awsDebug)
	fmt.Printf("Creating AWS resources. This should take around 5 minutes.\n")
	if err := cluster.Create(string(data), upOpts.s3URI); err != nil {
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
