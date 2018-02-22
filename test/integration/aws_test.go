package integration

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	"github.com/kubernetes-incubator/kube-aws/model"
	"github.com/kubernetes-incubator/kube-aws/test/helper"
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
	encryptService                config.EncryptService
}

func newKubeAwsSettingsFromEnv(t *testing.T) kubeAwsSettings {
	env := testEnv{t: t}

	clusterName, clusterNameExists := os.LookupEnv("KUBE_AWS_CLUSTER_NAME")

	if !clusterNameExists || clusterName == "" {
		clusterName = "it"
		t.Logf(`Falling back clusterName to a stub value "%s" for tests of validating stack templates. No assets will actually be uploaded to S3 and no clusters will be created with CloudFormation`, clusterName)
	}

	if useRealAWS() {
		externalDnsName := fmt.Sprintf("%s.%s", clusterName, env.get("KUBE_AWS_DOMAIN"))
		keyName := env.get("KUBE_AWS_KEY_NAME")
		kmsKeyArn := env.get("KUBE_AWS_KMS_KEY_ARN")
		region := env.get("KUBE_AWS_REGION")
		return kubeAwsSettings{
			clusterName:                   clusterName,
			etcdNodeDefaultInternalDomain: model.RegionForName(region).PrivateDomainName(),
			externalDNSName:               externalDnsName,
			keyName:                       keyName,
			kmsKeyArn:                     kmsKeyArn,
			region:                        region,
		}
	} else {
		return kubeAwsSettings{
			clusterName:                   clusterName,
			etcdNodeDefaultInternalDomain: model.RegionForName("us-west-1").PrivateDomainName(),
			externalDNSName:               "test.staging.core-os.net",
			keyName:                       "test-key-name",
			kmsKeyArn:                     "arn:aws:kms:us-west-1:xxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx",
			region:                        "us-west-1",
			encryptService:                helper.DummyEncryptService{},
		}
	}
}

func (s kubeAwsSettings) mainClusterYaml() string {
	return fmt.Sprintf(`clusterName: %s
apiEndpoints:
- name: public
  dnsName: "%s"
  loadBalancer:
    hostedZone:
      id: hostedzone-xxxx
keyName: "%s"
kmsKeyArn: "%s"
region: "%s"
`,
		s.clusterName,
		s.externalDNSName,
		s.keyName,
		s.kmsKeyArn,
		s.region,
	)
}

func (s kubeAwsSettings) mainClusterYamlWithoutAPIEndpoint() string {
	return fmt.Sprintf(`clusterName: %s
keyName: "%s"
kmsKeyArn: "%s"
region: "%s"
`,
		s.clusterName,
		s.keyName,
		s.kmsKeyArn,
		s.region,
	)
}

func (s kubeAwsSettings) minimumValidClusterYaml() string {
	return s.minimumValidClusterYamlWithAZ("a")
}

func (s kubeAwsSettings) minimumValidClusterYamlWithAZ(suffix string) string {
	return s.mainClusterYamlWithoutAPIEndpoint() + fmt.Sprintf(`
availabilityZone: %s
apiEndpoints:
- name: public
  dnsName: "%s"
  loadBalancer:
    hostedZone:
      id: hostedzone-xxxx
`, s.region+suffix, s.externalDNSName)
}

func (s kubeAwsSettings) withClusterName(n string) kubeAwsSettings {
	s.clusterName = n
	return s
}

func (s kubeAwsSettings) withRegion(r string) kubeAwsSettings {
	s.region = r
	return s
}
