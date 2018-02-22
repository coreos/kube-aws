package config

import (
	cfg "github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	"testing"
)

const cluster_config = `
availabilityZone: us-west-1c
keyName: test-key-name
region: us-west-1
clusterName: test-cluster-name
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
