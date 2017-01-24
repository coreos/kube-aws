package nodepool

import (
	"fmt"
	"io/ioutil"
	"os"

	"path"

	cfg "github.com/coreos/kube-aws/config"
	"github.com/coreos/kube-aws/nodepool/config"
	"github.com/spf13/cobra"
	"strconv"
	"strings"
)

var (
	cmdRender = &cobra.Command{
		Use:          "render",
		Short:        "Render deployment artifacts",
		Long:         ``,
		SilenceUsage: true,
	}

	cmdRenderStack = &cobra.Command{
		Use:          "stack",
		Short:        "Render CloudFormation stack template and coreos-cloudinit userdata",
		Long:         ``,
		RunE:         runCmdRenderStack,
		SilenceUsage: true,
	}
)

func init() {
	NodePoolCmd.AddCommand(cmdRender)

	cmdRender.AddCommand(cmdRenderStack)
}

func runCmdRenderStack(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("render stack takes no arguments\n")
	}

	// Validate flags.
	required := []struct {
		name, val string
	}{
		{"--node-pool-name", nodePoolOpts.PoolName},
	}
	var missing []string
	for _, req := range required {
		if req.val == "" {
			missing = append(missing, strconv.Quote(req.name))
		}
	}
	if len(missing) != 0 {
		return fmt.Errorf("Missing required flag(s): %s", strings.Join(missing, ", "))
	}

	// Write all assets to disk.
	files := []struct {
		name string
		data []byte
		mode os.FileMode
	}{
		{stackRenderOptions().WorkerTmplFile, cfg.CloudConfigWorker, 0644},
		{stackRenderOptions().StackTemplateTmplFile, config.StackTemplateTemplate, 0644},
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
		`Success! Stack rendered to node-pools/%s/stack-template.json.

Next steps:
1. (Optional) Validate your changes to %s with "kube-aws node-pools validate --node-pool-name %s"
2. (Optional) Further customize the cluster by modifying stack-template.json or files in ./userdata.
3. Start the cluster with "kube-aws up".
`

	fmt.Printf(successMsg, nodePoolOpts.PoolName, nodePoolClusterConfigFilePath(), nodePoolOpts.PoolName)
	return nil
}
