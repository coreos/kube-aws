package integration

import (
	"fmt"
	"github.com/coreos/kube-aws/config"
	"github.com/coreos/kube-aws/test/helper"
	"os"
	"testing"
)

type testEnv struct {
	t *testing.T
}

func (r testEnv) get(name string) string {
	v := os.Getenv(name)

	if v == "" {
		r.t.Errorf("%s must be set", name)
		r.t.FailNow()
	}

	return v
}

const minimalMainClusterYaml = `clusterName: test-cluster-name
externalDNSName: test.staging.core-os.net
keyName: test-key-name
kmsKeyArn: "arn:aws:kms:us-west-1:xxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx"
region: us-west-1
`

func useRealAWS() bool {
	return os.Getenv("KUBE_AWS_INTEGRATION_TEST") != ""
}

type kubeAwsSettings struct {
	clusterName     string
	externalDNSName string
	keyName         string
	kmsKeyArn       string
	region          string
	mainClusterYaml string
	encryptService  config.EncryptService
}

func newKubeAwsSettingsFromEnv(t *testing.T) kubeAwsSettings {
	env := testEnv{t: t}

	if useRealAWS() {
		clusterName := env.get("KUBE_AWS_CLUSTER_NAME")
		externalDnsName := fmt.Sprintf("%s.%s", clusterName, env.get("KUBE_AWS_DOMAIN"))
		keyName := env.get("KUBE_AWS_KEY_NAME")
		kmsKeyArn := env.get("KUBE_AWS_KMS_KEY_ARN")
		region := env.get("KUBE_AWS_REGION")
		yaml := fmt.Sprintf(`clusterName: %s
externalDNSName: "%s"
keyName: "%s"
kmsKeyArn: "%s"
region: "%s"
`,
			clusterName,
			externalDnsName,
			keyName,
			kmsKeyArn,
			region,
		)
		return kubeAwsSettings{
			clusterName:     clusterName,
			externalDNSName: externalDnsName,
			keyName:         keyName,
			kmsKeyArn:       kmsKeyArn,
			region:          region,
			mainClusterYaml: yaml,
		}
	} else {
		return kubeAwsSettings{
			mainClusterYaml: minimalMainClusterYaml,
			encryptService:  helper.DummyEncryptService{},
		}
	}
}
