package cmd

import (
	"fmt"
	"os"

	"github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	"github.com/kubernetes-incubator/kube-aws/core/root"
	"github.com/spf13/cobra"
)

var (
	cmdRender = &cobra.Command{
		Use:          "render",
		Short:        "Render deployment artifacts",
		Long:         ``,
		RunE:         runCmdRender,
		SilenceUsage: true,
	}

	cmdRenderTLSCredentials = &cobra.Command{
		Use:          "credentials",
		Short:        "Render TLS credentials",
		Long:         ``,
		RunE:         runCmdRenderTLSCredentials,
		SilenceUsage: true,
	}

	renderTLSCredentialsOpts = config.CredentialsOptions{}

	cmdRenderStack = &cobra.Command{
		Use:          "stack",
		Short:        "Render CloudFormation stack template and coreos-cloudinit userdata",
		Long:         ``,
		RunE:         runCmdRenderStack,
		SilenceUsage: true,
	}
)

func init() {
	RootCmd.AddCommand(cmdRender)

	cmdRender.AddCommand(cmdRenderTLSCredentials)
	cmdRender.AddCommand(cmdRenderStack)

	cmdRenderTLSCredentials.Flags().BoolVar(&renderTLSCredentialsOpts.GenerateCA, "generate-ca", false, "if generating credentials, generate root CA key and cert. NOT RECOMMENDED FOR PRODUCTION USE- use '-ca-key-path' and '-ca-cert-path' options to provide your own certificate authority assets")
	cmdRenderTLSCredentials.Flags().StringVar(&renderTLSCredentialsOpts.CaKeyPath, "ca-key-path", "./credentials/ca-key.pem", "path to pem-encoded CA RSA key")
	cmdRenderTLSCredentials.Flags().StringVar(&renderTLSCredentialsOpts.CaCertPath, "ca-cert-path", "./credentials/ca.pem", "path to pem-encoded CA x509 certificate")
}
func runCmdRender(cmd *cobra.Command, args []string) error {
	fmt.Println("WARNING: 'kube-aws render' is deprecated. See 'kube-aws render --help' for usage")
	if len(args) != 0 {
		return fmt.Errorf("render takes no arguments\n")
	}

	if _, err := os.Stat(renderTLSCredentialsOpts.CaKeyPath); os.IsNotExist(err) {
		renderTLSCredentialsOpts.GenerateCA = true
	}
	if err := runCmdRenderTLSCredentials(cmdRenderTLSCredentials, args); err != nil {
		return err
	}

	if err := runCmdRenderStack(cmdRenderTLSCredentials, args); err != nil {
		return err
	}

	return nil
}
func runCmdRenderStack(cmd *cobra.Command, args []string) error {
	// Read the config from file.
	cluster, err := root.StackAssetsRendererFromFile(configPath)
	if err != nil {
		return fmt.Errorf("Failed to read cluster config: %v", err)
	}

	if err := cluster.RenderFiles(); err != nil {
		return err
	}

	successMsg :=
		`Success! Stack rendered to ./stack-templates.

Next steps:
1. (Optional) Validate your changes to %s with "kube-aws validate"
2. (Optional) Further customize the cluster by modifying templates in ./stack-templates or cloud-configs in ./userdata.
3. Start the cluster with "kube-aws up".
`

	fmt.Printf(successMsg, configPath)
	return nil
}

func runCmdRenderTLSCredentials(cmd *cobra.Command, args []string) error {
	cluster, err := root.CredentialsRendererFromFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read cluster config: %v", err)
	}
	return cluster.RenderTLSCerts(renderTLSCredentialsOpts)
}
