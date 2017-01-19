package cmd

import (
	"fmt"

	"github.com/coreos/kube-aws/cluster"
	"github.com/coreos/kube-aws/config"
	"github.com/spf13/cobra"
)

var (
	cmdStatus = &cobra.Command{
		Use:          "status",
		Short:        "Describe an existing Kubernetes cluster",
		Long:         ``,
		RunE:         runCmdStatus,
		SilenceUsage: true,
	}
)

func init() {
	RootCmd.AddCommand(cmdStatus)
}

func runCmdStatus(cmd *cobra.Command, args []string) error {
	conf, err := config.ClusterFromFile(configPath)
	if err != nil {
		return fmt.Errorf("Failed to read cluster config: %v", err)
	}

	// call it here purely to make it compile
	stackConf, err := conf.RenderStackTemplate(stackTemplateOptions, upOpts.export, "")
	if err != nil {
		return fmt.Errorf("Failed to render stack template: %v", err)
	}

	info, err := cluster.New(stackConf, false).Info()
	if err != nil {
		return fmt.Errorf("Failed fetching cluster info: %v", err)
	}

	fmt.Print(info.String())
	return nil
}
