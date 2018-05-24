package cmd

import (
	"os"

	"github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	"github.com/kubernetes-incubator/kube-aws/core/root"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/spf13/cobra"
)

var (
	cmdRender = &cobra.Command{
		Use:          "render",
		Short:        "Render deployment artifacts",
		Long:         ``,
		Run:          runCmdRender,
		SilenceUsage: true,
	}

	cmdRenderCredentials = &cobra.Command{
		Use:          "credentials",
		Short:        "Render credentials",
		Long:         ``,
		Run:          runCmdRenderCredentials,
		SilenceUsage: true,
	}

	renderCredentialsOpts = config.CredentialsOptions{}

	cmdRenderStack = &cobra.Command{
		Use:          "stack",
		Short:        "Render CloudFormation stack template and coreos-cloudinit userdata",
		Long:         ``,
		Run:          runCmdRenderStack,
		SilenceUsage: true,
	}
)

func init() {
	RootCmd.AddCommand(cmdRender)

	cmdRender.AddCommand(cmdRenderCredentials)
	cmdRender.AddCommand(cmdRenderStack)

	cmdRenderCredentials.Flags().BoolVar(&renderCredentialsOpts.GenerateCA, "generate-ca", false, "if generating credentials, generate root CA key and cert. NOT RECOMMENDED FOR PRODUCTION USE- use '-ca-key-path' and '-ca-cert-path' options to provide your own certificate authority assets")
	cmdRenderCredentials.Flags().StringVar(&renderCredentialsOpts.CaKeyPath, "ca-key-path", "./credentials/ca-key.pem", "path to pem-encoded CA RSA key")
	cmdRenderCredentials.Flags().StringVar(&renderCredentialsOpts.CaCertPath, "ca-cert-path", "./credentials/ca.pem", "path to pem-encoded CA x509 certificate")
	cmdRenderCredentials.Flags().BoolVar(&renderCredentialsOpts.KIAM, "kiam", true, "generate TLS assets for kiam")
}
func runCmdRender(_ *cobra.Command, args []string) {
	logger.Warn("WARNING: 'kube-aws render' is deprecated. See 'kube-aws render --help' for usage")
	if len(args) != 0 {
		logger.Fatal("render takes no arguments\n")
	}

	if _, err := os.Stat(renderCredentialsOpts.CaKeyPath); os.IsNotExist(err) {
		renderCredentialsOpts.GenerateCA = true
	}
	runCmdRenderCredentials(cmdRenderCredentials, args)
	runCmdRenderStack(cmdRenderCredentials, args)
}

func runCmdRenderStack(_ *cobra.Command, _ []string) {
	if err := root.RenderStack(configPath); err != nil {
		logger.Fatal(err)
	}

	successMsg :=
		`Success! Stack rendered to ./stack-templates.

Next steps:
1. (Optional) Validate your changes to %s with "kube-aws validate"
2. (Optional) Further customize the cluster by modifying templates in ./stack-templates or cloud-configs in ./userdata.
3. Start the cluster with "kube-aws up".
`
	logger.Infof(successMsg, configPath)
}

func runCmdRenderCredentials(_ *cobra.Command, _ []string) {
	if err := root.RenderCredentials(configPath, renderCredentialsOpts); err != nil {
		logger.Fatal(err)
	}
}
