package api

import (
	"fmt"
	"github.com/aws/amazon-vpc-cni-k8s/pkg/awsutils"
	"github.com/kubernetes-incubator/kube-aws/provisioner"
)

type AmazonVPC struct {
	Enabled bool `yaml:"enabled"`
}

func (a AmazonVPC) MaxPodsScript() provisioner.Content {
	script := `#!/usr/bin/env bash

set -e

declare -A instance_eni_available
`

	for it, num := range awsutils.InstanceENIsAvailable {
		script = script + fmt.Sprintf(`instance_eni_available["%s"]=%d
`, it, num)
	}

	script = script + `
declare -A instance_ip_available
`
	for it, num := range awsutils.InstanceIPsAvailable {
		script = script + fmt.Sprintf(`instance_ip_available["%s"]=%d
`, it, num)
	}

	script = script + `

instance_type=$(curl http://169.254.169.254/latest/meta-data/instance-type)

enis=${instance_eni_available["$instance_type"]}

if [ "" == "$enis" ]; then
  echo "unsupported instance type: no enis_per_eni defined: $instance_type" 1>&2
  exit 1
fi

# According to https://github.com/aws/amazon-vpc-cni-k8s#eni-allocation
ips_per_eni=${instance_ip_available["$instance_type"]}

if [ "" == "$ips_per_eni" ]; then
  echo "unsupported instance type: no ips_per_eni defined: $instance_type" 1>&2
  exit 1
fi

max_pods=$(( (enis * (ips_per_eni - 1)) + 2 ))

printf $max_pods
`
	return provisioner.NewBinaryContent([]byte(script))
}
