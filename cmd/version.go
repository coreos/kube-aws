package cmd

import (
	"fmt"

	"github.com/kubernetes-incubator/kube-aws/core/controlplane/cluster"
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

func runCmdVersion(cmd *cobra.Command, args []string) {
	fmt.Printf("kube-aws version %s\n", cluster.VERSION)
}
