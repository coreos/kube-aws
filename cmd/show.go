package cmd

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/core/root"
	"github.com/kubernetes-incubator/kube-aws/tlscerts"
	"github.com/spf13/cobra"
	"sort"
)

var (
	cmdShow = &cobra.Command{
		Use:          "show",
		Short:        "Show info about certificates in credentials directory",
		Long:         ``,
		SilenceUsage: true,
	}

	cmdShowCertificates = &cobra.Command{
		Use:   "certificates",
		Short: "Show info about certificates",
		Long: `Loads all certificates from credentials directory and prints certificate
Issuer, Validity, Subject and DNS Names fields`,
		RunE:         runCmdShowCertificates,
		SilenceUsage: true,
	}
)

func init() {
	RootCmd.AddCommand(cmdShow)
	cmdShow.AddCommand(cmdShowCertificates)
}

func runCmdShowCertificates(_ *cobra.Command, _ []string) error {
	certs, err := root.LoadCertificates()
	if err != nil {
		return err
	}

	keys := sortedKeys(certs)
	for _, k := range keys {
		cert := certs[k]
		fmt.Printf("--- %s ---\n", k)
		for _, v := range cert {
			fmt.Println(v)
		}
		fmt.Println("")
	}
	return nil
}

func sortedKeys(m map[string]tlscerts.Certificates) []string {

	var keys []string
	for k := range m {
		keys = append(keys, k)
	}

	sort.Sort(sort.StringSlice(keys))
	return keys
}
