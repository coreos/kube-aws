package nodepool

import (
	"fmt"
	"strconv"
	"strings"

	cfg "github.com/coreos/kube-aws/config"
	"github.com/coreos/kube-aws/filegen"
	"github.com/coreos/kube-aws/nodepool/config"
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

	initOpts = config.ComputedConfig{}
)

func init() {
	NodePoolCmd.AddCommand(cmdInit)
	cmdInit.Flags().StringVar(&initOpts.AvailabilityZone, "availability-zone", "", "The AWS availability-zone to deploy to")
	cmdInit.Flags().StringVar(&initOpts.KeyName, "key-name", "", "The AWS key-pair for ssh access to nodes")
	cmdInit.Flags().StringVar(&initOpts.KMSKeyARN, "kms-key-arn", "", "The ARN of the AWS KMS key for encrypting TLS assets")
	cmdInit.Flags().StringVar(&initOpts.AmiId, "ami-id", "", "The AMI ID of CoreOS")
}

func runCmdInit(cmd *cobra.Command, args []string) error {
	initOpts.NodePoolName = nodePoolOpts.PoolName

	// Validate flags.
	required := []struct {
		name, val string
	}{
		{"--node-pool-name", initOpts.NodePoolName},
		{"--availability-zone", initOpts.AvailabilityZone},
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

	// Read the config from file.
	mainConfig, err := cfg.ClusterFromFile(clusterConfigFilePath())
	if err != nil {
		return fmt.Errorf("Failed to read cluster config: %v", err)
	}

	main, err := mainConfig.Config()
	if err != nil {
		return fmt.Errorf("Failed to create config: %v", err)
	}

	// Required and inheritable settings for the node pool.
	//
	// These can be set via command-line options to kube-aws nodepool init.
	// If omitted, these can be inherited from the main cluster's cluster.yaml.

	if initOpts.Region == "" {
		initOpts.Region = main.Region
	}

	if initOpts.KeyName == "" {
		initOpts.KeyName = main.KeyName
	}

	if initOpts.KMSKeyARN == "" {
		initOpts.KMSKeyARN = main.KMSKeyARN
	}

	if initOpts.AmiId == "" {
		initOpts.AmiId = main.AmiId
	}

	if initOpts.ReleaseChannel == "" {
		initOpts.ReleaseChannel = main.ReleaseChannel
	}

	if initOpts.VPCCIDR == "" {
		initOpts.VPCCIDR = main.VPCCIDR
	}

	// Required, inheritable and importable settings for the node pool.
	//
	// These can be customized in the node pool's cluster.yaml
	// If omitted from it, these can also can be exported from the main cluster
	// and then imported to the node pool in the cloudformation layer.

	if initOpts.VPCID == "" {
		initOpts.VPCID = main.VPCID
	}

	if initOpts.RouteTableID == "" {
		initOpts.RouteTableID = main.RouteTableID
	}

	if initOpts.EtcdEndpoints == "" {
		initOpts.EtcdEndpoints = main.EtcdEndpoints
	}

	initOpts.ClusterName = main.ClusterName

	// Required and shared settings for the node pool.
	//
	// These can not be customized in the node pool's cluster yaml.
	// Customizing these values in each node pool doesn't make sense
	// because inconsistency between the main cluster and node pools result in
	// unusable worker nodes that can't communicate with k8s apiserver, kube-dns, calico, etc.

	initOpts.KubeClusterSettings = main.KubeClusterSettings

	if err := filegen.CreateFileFromTemplate(nodePoolClusterConfigFilePath(), initOpts, config.DefaultClusterConfig); err != nil {
		return fmt.Errorf("Error exec-ing default config template: %v", err)
	}

	successMsg :=
		`Success! Created %s

Next steps:
1. (Optional) Edit %s to parameterize the cluster.
2. Use the "kube-aws node-pools render stack" command to render the CloudFormation stack template and coreos-cloudinit userdata.
`

	fmt.Printf(successMsg, nodePoolClusterConfigFilePath(), nodePoolClusterConfigFilePath())
	return nil
}
