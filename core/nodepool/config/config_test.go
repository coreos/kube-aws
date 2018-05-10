package config

import (
	"strings"
	"testing"

	cfg "github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
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

func TestRotateCerts(t *testing.T) {
	controlplane_config, _ := cfg.ConfigFromBytes([]byte(cluster_config))

	config, _ := ClusterFromBytes([]byte(""), controlplane_config)

	if !(config.FeatureGates()["RotateKubeletClientCertificate"] == "true") {
		t.Errorf("When RotateCerts is enabled, Feature Gate RotateKubeletClientCertificate should be automatically enabled too")
	}

}

func TestKube2IamKiamClash(t *testing.T) {
	config := `
name: nodepool1
kube2IamSupport:
  enabled: true
kiamSupport:
  enabled: true
`

	controlplane_config, _ := cfg.ConfigFromBytes([]byte(cluster_config))
	_, err := ClusterFromBytes([]byte(config), controlplane_config)
	if err == nil || !strings.Contains(err.Error(), "not both") {
		t.Errorf("expected config to cause error as kube2iam and kiam cannot be enabled together: %s\n%s", err, config)
	}
}
