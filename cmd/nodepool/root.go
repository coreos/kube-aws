package nodepool

import (
	"fmt"
	"github.com/coreos/kube-aws/nodepool/config"
	"github.com/spf13/cobra"
)

var (
	NodePoolCmd = &cobra.Command{
		Use:   "node-pools",
		Short: "Manage node pools",
		Long:  ``,
	}

	nodePoolOpts = config.Ref{}
)

func clusterConfigFilePath() string {
	return "cluster.yaml"
}

func nodePoolConfigDirPath() string {
	return fmt.Sprintf("node-pools/%s", nodePoolOpts.PoolName)
}

func nodePoolClusterConfigFilePath() string {
	return fmt.Sprintf("%s/cluster.yaml", nodePoolConfigDirPath())
}

func nodePoolExportedStackTemplatePath() string {
	return fmt.Sprintf("%s/%s.stack-template.json", nodePoolConfigDirPath(), nodePoolOpts.PoolName)
}

func stackTemplateOptions() config.StackTemplateOptions {
	return config.StackTemplateOptions{
		TLSAssetsDir:          "credentials",
		WorkerTmplFile:        fmt.Sprintf("%s/userdata/cloud-config-worker", nodePoolConfigDirPath()),
		StackTemplateTmplFile: fmt.Sprintf("%s/stack-template.json", nodePoolConfigDirPath()),
	}
}

func init() {
	NodePoolCmd.PersistentFlags().StringVar(&nodePoolOpts.PoolName, "node-pool-name", "", "The name of this node pool. This will be the name of the cloudformation stack")
}
