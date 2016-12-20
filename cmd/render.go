package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"text/template"

	"crypto/rsa"
	"crypto/x509"

	"path"

	"github.com/coreos/kube-aws/config"
	"github.com/coreos/kube-aws/tlsutil"
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

	cmdRenderCredentials = &cobra.Command{
		Use:          "credentials",
		Short:        "Render TLS credentials",
		Long:         ``,
		RunE:         runCmdRenderCredentials,
		SilenceUsage: true,
	}

	renderCredentialsOpts = struct {
		generateCA bool
		caKeyPath  string
		caCertPath string
	}{}

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

	cmdRender.AddCommand(cmdRenderCredentials)
	cmdRenderCredentials.Flags().BoolVar(&renderCredentialsOpts.generateCA, "generate-ca", false, "if generating credentials, generate root CA key and cert. NOT RECOMMENDED FOR PRODUCTION USE- use '-ca-key-path' and '-ca-cert-path' options to provide your own certificate authority assets")
	cmdRenderCredentials.Flags().StringVar(&renderCredentialsOpts.caKeyPath, "ca-key-path", "./credentials/ca-key.pem", "path to pem-encoded CA RSA key")
	cmdRenderCredentials.Flags().StringVar(&renderCredentialsOpts.caCertPath, "ca-cert-path", "./credentials/ca.pem", "path to pem-encoded CA x509 certificate")

	cmdRender.AddCommand(cmdRenderStack)
}
func runCmdRender(cmd *cobra.Command, args []string) error {
	fmt.Printf("WARNING: 'kube-aws render' is deprecated. See 'kube-aws render --help' for usage\n")
	if len(args) != 0 {
		return fmt.Errorf("render takes no arguments\n")
	}

	if _, err := os.Stat(renderCredentialsOpts.caKeyPath); os.IsNotExist(err) {
		renderCredentialsOpts.generateCA = true
	}
	if err := runCmdRenderCredentials(cmdRenderCredentials, args); err != nil {
		return err
	}

	if err := runCmdRenderStack(cmdRenderCredentials, args); err != nil {
		return err
	}

	return nil
}
func runCmdRenderStack(cmd *cobra.Command, args []string) error {
	// Read the config from file.
	cluster, err := config.ClusterFromFile(configPath)
	if err != nil {
		return fmt.Errorf("Failed to read cluster config: %v", err)
	}

	// Create a Config and attempt to render a kubeconfig for it.
	cfg, err := cluster.Config()
	if err != nil {
		return fmt.Errorf("Failed to create config: %v", err)
	}
	tmpl, err := template.New("kubeconfig.yaml").Parse(string(config.KubeConfigTemplate))
	if err != nil {
		return fmt.Errorf("Failed to parse default kubeconfig template: %v", err)
	}
	var kubeconfig bytes.Buffer
	if err := tmpl.Execute(&kubeconfig, cfg); err != nil {
		return fmt.Errorf("Failed to render kubeconfig: %v", err)
	}

	// Write all assets to disk.
	files := []struct {
		name string
		data []byte
		mode os.FileMode
	}{
		{"credentials/.gitignore", []byte("*"), 0644},
		{"userdata/cloud-config-controller", config.CloudConfigController, 0644},
		{"userdata/cloud-config-worker", config.CloudConfigWorker, 0644},
		{"userdata/cloud-config-etcd", config.CloudConfigEtcd, 0644},
		{"stack-template.json", config.StackTemplateTemplate, 0644},
		{"kubeconfig", kubeconfig.Bytes(), 0600},
	}
	for _, file := range files {
		if err := os.MkdirAll(path.Dir(file.name), 0755); err != nil {
			return err
		}

		if err := ioutil.WriteFile(file.name, file.data, file.mode); err != nil {
			return err
		}
	}

	successMsg :=
		`Success! Stack rendered to stack-template.json.

Next steps:
1. (Optional) Validate your changes to %s with "kube-aws validate"
2. (Optional) Further customize the cluster by modifying stack-template.json or files in ./userdata.
3. Start the cluster with "kube-aws up".
`

	fmt.Printf(successMsg, configPath)
	return nil
}

func runCmdRenderCredentials(cmd *cobra.Command, args []string) error {
	cluster, err := config.ClusterFromFile(configPath)
	if err != nil {
		return fmt.Errorf("Failed to read cluster config: %v", err)
	}

	fmt.Printf("Generating TLS credentials...\n")
	var caKey *rsa.PrivateKey
	var caCert *x509.Certificate
	if renderCredentialsOpts.generateCA {
		var err error
		caKey, caCert, err = cluster.NewTLSCA()
		if err != nil {
			return fmt.Errorf("failed generating cluster CA: %v", err)
		}
		fmt.Printf("-> Generating new TLS CA\n")
	} else {
		fmt.Printf("-> Parsing existing TLS CA\n")
		if caKeyBytes, err := ioutil.ReadFile(renderCredentialsOpts.caKeyPath); err != nil {
			return fmt.Errorf("failed reading ca key file %s : %v", renderCredentialsOpts.caKeyPath, err)
		} else {
			if caKey, err = tlsutil.DecodePrivateKeyPEM(caKeyBytes); err != nil {
				return fmt.Errorf("failed parsing ca key: %v", err)
			}
		}
		if caCertBytes, err := ioutil.ReadFile(renderCredentialsOpts.caCertPath); err != nil {
			return fmt.Errorf("failed reading ca cert file %s : %v", renderCredentialsOpts.caCertPath, err)
		} else {
			if caCert, err = tlsutil.DecodeCertificatePEM(caCertBytes); err != nil {
				return fmt.Errorf("failed parsing ca cert: %v", err)
			}
		}
	}
	fmt.Printf("-> Generating new TLS assets\n")
	assets, err := cluster.NewTLSAssets(caKey, caCert)
	if err != nil {
		return fmt.Errorf("Error generating default assets: %v", err)
	}
	if err := os.MkdirAll("credentials", 0700); err != nil {
		return err
	}
	if err := assets.WriteToDir("./credentials", renderCredentialsOpts.generateCA); err != nil {
		return fmt.Errorf("Error create assets: %v", err)
	}

	return nil
}
