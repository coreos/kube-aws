package nodepool

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/coreos/kube-aws/nodepool/cluster"
	"github.com/coreos/kube-aws/nodepool/config"
)

var (
	cmdDestroy = &cobra.Command{
		Use:          "destroy",
		Short:        "Destroy an existing node pool",
		Long:         ``,
		RunE:         runCmdDestroy,
		SilenceUsage: true,
	}
	destroyOpts = struct {
		awsDebug bool
	}{}
)

func init() {
	NodePoolCmd.AddCommand(cmdDestroy)
	cmdDestroy.Flags().BoolVar(&destroyOpts.awsDebug, "aws-debug", false, "Log debug information from aws-sdk-go library")
}

func runCmdDestroy(cmd *cobra.Command, args []string) error {
	cfg, err := config.ClusterFromFile(nodePoolClusterConfigFilePath())
	if err != nil {
		return fmt.Errorf("Error parsing node pool config: %v", err)
	}

	// call it here purely to make it compile
        stackConf, err := cfg.RenderStackTemplate(stackTemplateOptions(), upOpts.export, "")
        if err != nil {
                return fmt.Errorf("Failed to render stack template: %v", err)
        }

	c := cluster.New(stackConf, destroyOpts.awsDebug)
	if err := c.Destroy(); err != nil {
		return fmt.Errorf("Failed destroying node pool: %v", err)
	}

	fmt.Println("CloudFormation stack is being destroyed. This will take several minutes")
	return nil
}
