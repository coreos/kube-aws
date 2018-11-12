package model

import (
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"github.com/stretchr/testify/assert"
	"testing"
)

const cluster_config = `
availabilityZone: us-west-1c
keyName: test-key-name
region: us-west-1
clusterName: test-cluster-name
s3URI: s3://bucket/demo
kmsKeyArn: "arn:aws:kms:us-west-1:xxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx"
apiEndpoints:
- name: public
  dnsName: test.staging.core-os.net
  loadBalancer:
    hostedZone:
      id: hostedzone-xxxxxx

kubelet:
  rotateCerts: 
    enabled: true
`

func ConfigFromBytes(data []byte) (*Config, error) {
	c, err := ClusterFromBytes(data)
	if err != nil {
		return nil, err
	}
	opts := api.ClusterOptions{
		S3URI: c.S3URI,
		// TODO
		SkipWait: false,
	}

	cpConfig, err := Compile(c, opts)
	if err != nil {
		return nil, err
	}

	return cpConfig, nil
}

func NodePoolConfigFromBytes(data []byte) (*NodePoolConfig, error) {
	c, err := ConfigFromBytes(data)
	if err != nil {
		return nil, err
	}

	return NodePoolCompile(c.Worker.NodePools[0], c)
}

func TestNodePoolRotateCerts(t *testing.T) {
	npconfig := NodePoolConfig{
		WorkerNodePool: api.WorkerNodePool{
			Kubelet: api.Kubelet{
				RotateCerts: api.RotateCerts{
					Enabled: true,
				},
			},
		},
	}

	if !(npconfig.FeatureGates()["RotateKubeletClientCertificate"] == "true") {
		t.Errorf("When RotateCerts is enabled, Feature Gate RotateKubeletClientCertificate should be automatically enabled too")
	}
}

const externalDNSNameConfig = `externalDNSName: test.staging.core-os.net
`

const availabilityZoneConfig = `availabilityZone: us-west-1c
`

const apiEndpointMinimalConfigYaml = `keyName: test-key-name
region: us-west-1
s3URI: s3://mybucket/mydir
clusterName: test-cluster-name
kmsKeyArn: "arn:aws:kms:us-west-1:xxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx"
`
const minimalConfigYaml = externalDNSNameConfig + apiEndpointMinimalConfigYaml
const singleAzConfigYaml = minimalConfigYaml + availabilityZoneConfig

func TestWithTrailingDot(t *testing.T) {
	tests := [][]string{
		[]string{
			"",
			"",
		},
		[]string{
			"foo.bar.",
			"foo.bar.",
		},
		[]string{
			"foo.bar",
			"foo.bar.",
		},
	}

	for _, test := range tests {
		input := test[0]
		actual := WithTrailingDot(input)
		expected := test[1]

		if expected != actual {
			t.Errorf(
				"WithTrailingDot(\"%s\") expected to return \"%s\" but it returned: \"%s\"",
				input,
				expected,
				actual,
			)
		}
	}
}

func TestStackNameDefaults(t *testing.T) {
	yaml, err := genConfigYamlForTesting("")
	if err != nil {
		t.Errorf("%v", err)
		t.FailNow()
	}
	c, err := ConfigFromBytes([]byte(yaml))
	if err != nil {
		t.Errorf("could not get valid cluster config: %v\n%s", err, yaml)
		t.FailNow()
	}

	assert.Equal(t, "control-plane", c.ControlPlaneStackName(), "Invalid ControlPlane Stackname, should be set to 'control-plane' if no override is provided.")
	assert.Equal(t, "network", c.NetworkStackName(), "Invalid Network Stackname, should be set to 'network' if no override is provided.")
	assert.Equal(t, "etcd", c.EtcdStackName(), "Invalid Etcd Stackname, should be set to 'etcd' if no override is provided.")
}

func TestStackNameOverrides(t *testing.T) {
	stackNameOverrideConfig := `
cloudformation:
  stackNameOverrides:
    controlPlane: "control-plane-override"
    network: "network-override"
    etcd: "etcd-override"
`
	yaml, err := genConfigYamlForTesting(stackNameOverrideConfig)
	if err != nil {
		t.Errorf("%v", err)
		t.FailNow()
	}
	c, err := ConfigFromBytes([]byte(yaml))
	if err != nil {
		t.Errorf("could not get valid cluster config: %v\n%s", err, yaml)
		t.FailNow()
	}

	assert.Equal(t, "control-plane-override", c.ControlPlaneStackName(), "Invalid ControlPlane Stackname, should be overridden with 'control-plane-override'.")
	assert.Equal(t, "network-override", c.NetworkStackName(), "Invalid Network Stackname, should overridden to 'network-override'.")
	assert.Equal(t, "etcd-override", c.EtcdStackName(), "Invalid Etcd Stackname, should be overridden with 'etcd-override'.")
}
