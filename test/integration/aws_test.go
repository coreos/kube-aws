package integration

import (
	"fmt"
	"github.com/coreos/kube-aws/core/controlplane/config"
	"github.com/coreos/kube-aws/model"
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

func useRealAWS() bool {
	return os.Getenv("KUBE_AWS_INTEGRATION_TEST") != ""
}

type kubeAwsSettings struct {
	clusterName                   string
	etcdNodeDefaultInternalDomain string
	externalDNSName               string
	keyName                       string
	kmsKeyArn                     string
	region                        string
	mainClusterYaml               string
	encryptService                config.EncryptService
}

func newKubeAwsSettingsFromEnv(t *testing.T) kubeAwsSettings {
	env := testEnv{t: t}

	clusterName, clusterNameExists := os.LookupEnv("KUBE_AWS_CLUSTER_NAME")

	if !clusterNameExists || clusterName == "" {
		clusterName = "kubeaws-it"
		t.Logf(`Falling back clusterName to a stub value "%s" for tests of validating stack templates. No assets will actually be uploaded to S3 and no clusters will be created with CloudFormation`, clusterName)
	}

	if useRealAWS() {
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
			clusterName:                   clusterName,
			etcdNodeDefaultInternalDomain: model.RegionForName(region).PrivateDomainName(),
			externalDNSName:               externalDnsName,
			keyName:                       keyName,
			kmsKeyArn:                     kmsKeyArn,
			region:                        region,
			mainClusterYaml:               yaml,
		}
	} else {
		return kubeAwsSettings{
			clusterName: clusterName,
			mainClusterYaml: fmt.Sprintf(`clusterName: %s
externalDNSName: test.staging.core-os.net
keyName: test-key-name
kmsKeyArn: "arn:aws:kms:us-west-1:xxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx"
region: us-west-1
`, clusterName),
			encryptService:                helper.DummyEncryptService{},
			etcdNodeDefaultInternalDomain: model.RegionForName("us-west-1").PrivateDomainName(),
		}
	}
}
