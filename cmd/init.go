package cmd

import (
	"errors"
	"fmt"

	"github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	"github.com/kubernetes-incubator/kube-aws/filegen"
	"github.com/spf13/cobra"
)

var (
	cmdInit = &cobra.Command{
		Use:          "init",
		Short:        "Initialize default node pool configuration",
		Long:         ``,
		RunE:         runCmdInit,
		SilenceUsage: true,
	}

	initOpts = config.InitialConfig{}
)

func init() {
	RootCmd.AddCommand(cmdInit)
	cmdInit.Flags().StringVar(&initOpts.ClusterName, "cluster-name", "", "The name of this cluster. This will be the name of the cloudformation stack")
	cmdInit.Flags().StringVar(&initOpts.ExternalDNSName, "external-dns-name", "", "The hostname that will route to the api server")
	cmdInit.Flags().StringVar(&initOpts.HostedZoneID, "hosted-zone-id", "", "The hosted zone in which a Route53 record set for a k8s API endpoint is created")
	cmdInit.Flags().StringVar(&initOpts.Region.Name, "region", "", "The AWS region to deploy to")
	cmdInit.Flags().StringVar(&initOpts.AvailabilityZone, "availability-zone", "", "The AWS availability-zone to deploy to")
	cmdInit.Flags().StringVar(&initOpts.KeyName, "key-name", "", "The AWS key-pair for ssh access to nodes")
	cmdInit.Flags().StringVar(&initOpts.KMSKeyARN, "kms-key-arn", "", "The ARN of the AWS KMS key for encrypting TLS assets")
	cmdInit.Flags().StringVar(&initOpts.AmiId, "ami-id", "", "The AMI ID of CoreOS")
	cmdInit.Flags().BoolVar(&initOpts.NoRecordSet, "no-record-set", false, "Instruct kube-aws to not manage Route53 record sets for your K8S API endpoints")
}

func runCmdInit(cmd *cobra.Command, args []string) error {
	// Validate flags.
	if err := validateRequired(
		flag{"--cluster-name", initOpts.ClusterName},
		flag{"--external-dns-name", initOpts.ExternalDNSName},
		flag{"--region", initOpts.Region.Name},
		flag{"--availability-zone", initOpts.AvailabilityZone},
	); err != nil {
		return err
	}

	if !initOpts.NoRecordSet && initOpts.HostedZoneID == "" {
		return errors.New("Missing required flags: either --hosted-zone-id or --no-record-set is required")
	}

	if err := filegen.CreateFileFromTemplate(configPath, initOpts, config.DefaultClusterConfig); err != nil {
		return fmt.Errorf("Error exec-ing default config template: %v", err)
	}

	successMsg :=
		`Success! Created %s

Next steps:
1. (Optional) Edit %s to parameterize the cluster.
2. Use the "kube-aws render" command to render the CloudFormation stack template and coreos-cloudinit userdata.
`

	fmt.Printf(successMsg, configPath, configPath)
	return nil
}
