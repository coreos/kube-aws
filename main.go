package main

import (
	"os"

	"github.com/kubernetes-incubator/kube-aws/cmd"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(2)
	}
}
