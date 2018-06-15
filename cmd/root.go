package cmd

import (
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/spf13/cobra"
)

var (
	RootCmd = &cobra.Command{
		Use:   "kube-aws",
		Short: "Manage Kubernetes clusters on AWS",
		Long:  ``,
	}

	configPath = "cluster.yaml"
)

func init() {
	RootCmd.SetOutput(logger.Writer(logger.StdErrOutput))
	RootCmd.PersistentFlags().BoolVarP(
		&logger.Silent,
		"silent",
		"s",
		false,
		"do not show messages",
	)
	RootCmd.PersistentFlags().BoolVarP(
		&logger.Verbose,
		"verbose",
		"v",
		false,
		"show debug messages",
	)
	RootCmd.PersistentFlags().BoolVarP(
		&logger.Color,
		"color",
		"",
		false,
		"use color for messages",
	)
}
