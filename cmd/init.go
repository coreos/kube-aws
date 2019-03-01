package cmd

import (
	"errors"
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/builtin"
	"github.com/kubernetes-incubator/kube-aws/coreos/amiregistry"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"github.com/kubernetes-incubator/kube-aws/tmpl"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type initialConfig struct {
	AmiId            string
	AvailabilityZone string
	ClusterName      string
	ExternalDNSName  string
	HostedZoneID     string
	KMSKeyARN        string
	KeyName          string
	NoRecordSet      bool
	Region           api.Region
	S3URI            string
}

var (
	cmdInit = &cobra.Command{
		Use:          "init",
		Short:        "Initialize default node pool configuration",
		Long:         ``,
		RunE:         runCmdInit,
		SilenceUsage: true,
	}

	initOpts = initialConfig{}
)

type flag struct {
	name string
	val  string
}

func validateRequiredFlags(required ...flag) error {
	var missing []string
	for _, req := range required {
		if req.val == "" {
			missing = append(missing, strconv.Quote(req.name))
		}
	}
	if len(missing) != 0 {
		return fmt.Errorf("missing required flag(s): %s", strings.Join(missing, ", "))
	}
	return nil
}

func init() {
	RootCmd.AddCommand(cmdInit)
	cmdInit.Flags().StringVar(&initOpts.S3URI, "s3-uri", "", "The URI of the S3 bucket")
	cmdInit.Flags().StringVar(&initOpts.ClusterName, "cluster-name", "", "The name of this cluster. This will be the name of the cloudformation stack")
	cmdInit.Flags().StringVar(&initOpts.ExternalDNSName, "external-dns-name", "", "The hostname that will route to the api server")
	cmdInit.Flags().StringVar(&initOpts.HostedZoneID, "hosted-zone-id", "", "The hosted zone in which a Route53 record set for a k8s API endpoint is created")
	cmdInit.Flags().StringVar(&initOpts.Region.Name, "region", "", "The AWS region to deploy to")
	cmdInit.Flags().StringVar(&initOpts.AvailabilityZone, "availability-zone", "", "The AWS availability-zone to deploy to")
	cmdInit.Flags().StringVar(&initOpts.KeyName, "key-name", "", "The AWS key-pair for ssh access to nodes")
	cmdInit.Flags().StringVar(&initOpts.KMSKeyARN, "kms-key-arn", "", "The ARN of the AWS KMS key for encrypting TLS assets")
	cmdInit.Flags().StringVar(&initOpts.AmiId, "ami-id", "", "The AMI ID of CoreOS. Last CoreOS Stable Channel selected by default if empty")
	cmdInit.Flags().BoolVar(&initOpts.NoRecordSet, "no-record-set", false, "Instruct kube-aws to not manage Route53 record sets for your K8S API endpoints")
}

func runCmdInit(_ *cobra.Command, _ []string) error {
	// Validate flags.
	if err := validateRequiredFlags(
		flag{"--s3-uri", initOpts.S3URI},
		flag{"--cluster-name", initOpts.ClusterName},
		flag{"--external-dns-name", initOpts.ExternalDNSName},
		flag{"--region", initOpts.Region.Name},
		flag{"--availability-zone", initOpts.AvailabilityZone},
	); err != nil {
		return err
	}

	if initOpts.AmiId == "" {
		defaultReleaseChannel := "stable"
		amiID, err := amiregistry.GetAMI(initOpts.Region.Name, defaultReleaseChannel)
		initOpts.AmiId = amiID
		if err != nil {
			return fmt.Errorf("cannot retrieve CoreOS AMI for region %s, channel %s", initOpts.Region.Name, defaultReleaseChannel)
		}
	}

	if !initOpts.NoRecordSet && initOpts.HostedZoneID == "" {
		return errors.New("missing required flags: either --hosted-zone-id or --no-record-set is required")
	}

	if err := createClusterConfigFromTemplate(configPath, builtin.String("cluster.yaml.tmpl"), initOpts); err != nil {
		return fmt.Errorf("error exec-ing default config template: %v", err)
	}

	successMsg :=
		`Success! Created %s

Next steps:
1. (Optional) Edit %s to parameterize the cluster.
2. Use the "kube-aws render" command to render the CloudFormation stack template and coreos-cloudinit userdata.
`
	logger.Infof(successMsg, configPath, configPath)
	return nil
}

func createClusterConfigFromTemplate(outputFilePath, fileTemplate string, templateOpts interface{}) error {
	dir := filepath.Dir(outputFilePath)

	if _, err := os.Stat(dir); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("error creating directory: %v", err)
		}
	}

	out, err := os.OpenFile(outputFilePath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("error opening %s : %v", outputFilePath, err)
	}
	defer out.Close()

	if err := tmpl.WriteTemplateWithOptions(out, fileTemplate, templateOpts); err != nil {
		return fmt.Errorf("cannot create cluster config from template: %v", err)
	}

	return nil
}
