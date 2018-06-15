package cmd

import (
	"github.com/kubernetes-incubator/kube-aws/core/controlplane/cluster"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/spf13/cobra"
)

var (
	cmdVersion = &cobra.Command{
		Use:   "version",
		Short: "Print version information and exit",
		Long:  ``,
		Run:   runCmdVersion,
	}
)

func init() {
	RootCmd.AddCommand(cmdVersion)
}

func runCmdVersion(_ *cobra.Command, _ []string) {
	logger.Infof("kube-aws version %s\n", cluster.VERSION)
}
