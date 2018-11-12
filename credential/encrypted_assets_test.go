package credential

import (
	"testing"

	"encoding/base64"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"fmt"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/kubernetes-incubator/kube-aws/test/helper"
)

var goodNetworkingConfigs = []string{
	``, //Tests validity of default network config values
	`
vpcCIDR: 10.4.3.0/24
instanceCIDR: 10.4.3.0/24
podCIDR: 172.4.0.0/16
serviceCIDR: 172.5.0.0/16
dnsServiceIP: 172.5.100.101
`, `
vpcCIDR: 10.4.0.0/16
instanceCIDR: 10.4.3.0/24
podCIDR: 10.6.0.0/16
serviceCIDR: 10.5.0.0/16
dnsServiceIP: 10.5.100.101
`, `
vpcId: vpc-xxxxx
routeTableId: rtb-xxxxxx
`, `
vpcId: vpc-xxxxx
`, `
createRecordSet: false
hostedZoneId: ""
`, `
createRecordSet: true
recordSetTTL: 400
hostedZoneId: "XXXXXXXXXXX"
`, `
createRecordSet: true
hostedZoneId: "XXXXXXXXXXX"
`,
}

type dummyEncryptService struct{}

var numEncryption = 0

func (d *dummyEncryptService) Encrypt(input *kms.EncryptInput) (*kms.EncryptOutput, error) {
	output := kms.EncryptOutput{
		CiphertextBlob: []byte(fmt.Sprintf("%s%d", string(input.Plaintext), numEncryption)),
	}
	numEncryption += 1
	return &output, nil
}

func TestReadOrCreateCompactAssets(t *testing.T) {
	helper.WithDummyCredentials(func(dir string) {
		kmsConfig := NewKMSConfig("keyarn", &dummyEncryptService{}, nil)

		// See https://github.com/kubernetes-incubator/kube-aws/issues/107
		t.Run("CachedToPreventUnnecessaryNodeReplacement", func(t *testing.T) {
			created, err := ReadOrCreateCompactAssets(dir, true, true, true, kmsConfig)

			if err != nil {
				t.Errorf("failed to read or update compact assets in %s : %v", dir, err)
				t.FailNow()
			}

			// This depends on TestDummyEncryptService which ensures dummy encrypt service to produce different ciphertext for each encryption
			// created == read means that encrypted assets were loaded from cached files named *.pem.enc, instead of re-encrypting raw assets named *.pem files
			// TODO Use some kind of mocking framework for tests like this
			read, err := ReadOrCreateCompactAssets(dir, true, true, true, kmsConfig)

			if err != nil {
				t.Errorf("failed to read or update compact assets in %s : %v", dir, err)
				t.FailNow()
			}

			if !reflect.DeepEqual(created, read) {
				t.Errorf(`failed to content encrypted assets.
	encrypted assets must not change after their first creation but they did change:
	created = %v
	read = %v`, created, read)
			}
		})

		t.Run("RemoveFilesToRegenerate", func(t *testing.T) {
			original, err := ReadOrCreateCompactAssets(dir, true, true, true, kmsConfig)

			if err != nil {
				t.Errorf("failed to read the original encrypted assets : %v", err)
				t.FailNow()
			}

			files := []string{
				"admin-key.pem.enc", "worker-key.pem.enc", "apiserver-key.pem.enc",
				"etcd-key.pem.enc", "etcd-client-key.pem.enc", "worker-ca-key.pem.enc",
				"kube-controller-manager-key.pem.enc", "kube-scheduler-key.pem.enc",
				"kiam-agent-key.pem.enc", "kiam-server-key.pem.enc", "apiserver-aggregator-key.pem.enc",
			}

			for _, filename := range files {
				if err := os.Remove(filepath.Join(dir, filename)); err != nil {
					t.Errorf("failed to remove %s for test setup : %v", filename, err)
					t.FailNow()
				}
			}

			regenerated, err := ReadOrCreateCompactAssets(dir, true, true, true, kmsConfig)

			if err != nil {
				t.Errorf("failed to read the regenerated encrypted assets : %v", err)
				t.FailNow()
			}

			for _, v := range [][]string{
				{"AdminCert", original.AdminCert, regenerated.AdminCert},
				{"CACert", original.CACert, regenerated.CACert},
				{"WorkerCert", original.WorkerCert, regenerated.WorkerCert},
				{"APIServerCert", original.APIServerCert, regenerated.APIServerCert},
				{"KubeControllerManagerCert", original.KubeControllerManagerCert, regenerated.KubeControllerManagerCert},
				{"KubeSchedulerCert", original.KubeSchedulerCert, regenerated.KubeSchedulerCert},
				{"EtcdClientCert", original.EtcdClientCert, regenerated.EtcdClientCert},
				{"EtcdCert", original.EtcdCert, regenerated.EtcdCert},
				{"KIAMAgentCert", original.KIAMAgentCert, regenerated.KIAMAgentCert},
				{"KIAMServerCert", original.KIAMServerCert, regenerated.KIAMServerCert},
				{"KIAMCACert", original.KIAMCACert, regenerated.KIAMCACert},
				{"APIServerAggregatorCert", original.APIServerAggregatorCert, regenerated.APIServerAggregatorCert},
			} {
				if v[1] != v[2] {
					t.Errorf("%s must NOT change but it did : original = %v, regenrated = %v ", v[0], v[1], v[2])
				}
			}

			for _, v := range [][]string{
				{"AdminKey", original.AdminKey, regenerated.AdminKey},
				{"WorkerCAKey", original.WorkerCAKey, regenerated.WorkerCAKey},
				{"WorkerKey", original.WorkerKey, regenerated.WorkerKey},
				{"APIServerKey", original.APIServerKey, regenerated.APIServerKey},
				{"KubeControllerManagerKey", original.KubeControllerManagerKey, regenerated.KubeControllerManagerKey},
				{"KubeSchedulerKey", original.KubeSchedulerKey, regenerated.KubeSchedulerKey},
				{"EtcdClientKey", original.EtcdClientKey, regenerated.EtcdClientKey},
				{"EtcdKey", original.EtcdKey, regenerated.EtcdKey},
				{"KIAMAgentKey", original.KIAMAgentKey, regenerated.KIAMAgentKey},
				{"KIAMServerKey", original.KIAMServerKey, regenerated.KIAMServerKey},
				{"APIServerAggregatorKey", original.APIServerAggregatorKey, regenerated.APIServerAggregatorKey},
			} {
				if v[1] == v[2] {
					t.Errorf("%s must change but it didn't : original = %v, regenrated = %v ", v[0], v[1], v[2])
				}
			}
			if reflect.DeepEqual(original, regenerated) {
				t.Errorf(`unexpecteed data contained in (possibly) regenerated encrypted assets.
	encrypted assets must change after regeneration but they didn't:
	original = %v
	regenerated = %v`, original, regenerated)
			}
		})
	})
}

func TestReadOrCreateUnEncryptedCompactAssets(t *testing.T) {
	run := func(dir string, caKeyRequiredOnController bool, t *testing.T) {
		t.Run("CachedToPreventUnnecessaryNodeReplacementOnUnencrypted", func(t *testing.T) {
			created, err := ReadOrCreateUnencryptedCompactAssets(dir, true, caKeyRequiredOnController, true)

			if err != nil {
				t.Errorf("failed to read or update compact assets in %s : %v", dir, err)
			}

			read, err := ReadOrCreateUnencryptedCompactAssets(dir, true, caKeyRequiredOnController, true)

			if err != nil {
				t.Errorf("failed to read or update compact assets in %s : %v", dir, err)
			}

			if !reflect.DeepEqual(created, read) {
				t.Errorf(`failed to content unencrypted assets.
		unencrypted assets must not change after their first creation but they did change:
		created = %v
		read = %v`, created, read)
			}
		})
	}

	t.Run("WithDummyCredentialsButCAKey", func(t *testing.T) {
		helper.WithDummyCredentialsButCAKey(func(dir string) {
			run(dir, false, t)
		})
	})
	t.Run("WithDummyCredentials", func(t *testing.T) {
		helper.WithDummyCredentials(func(dir string) {
			run(dir, true, t)
		})
	})
}

func TestRandomTokenString(t *testing.T) {
	randomToken, err := RandomTokenString()
	if err != nil {
		t.Errorf("failed to generate a Kubelet bootstrap token: %v", err)
	}
	if strings.Index(randomToken, ",") >= 0 {
		t.Errorf("random token not expect to contain a comma: %v", randomToken)
	}

	b, err := base64.StdEncoding.DecodeString(randomToken)
	if err != nil {
		t.Errorf("failed to decode base64 token string: %v", err)
	}
	if len(b) != 32 {
		t.Errorf("expected token to be 256 bits long, but was %d", len(b))
	}
}

func TestHasAuthTokens(t *testing.T) {
	testCases := []struct {
		authTokens string
		expected   bool
	}{
		// Without auth tokens
		{
			authTokens: "",
			expected:   false,
		},

		// With auth tokens
		{
			authTokens: "contents",
			expected:   true,
		},
	}

	for _, testCase := range testCases {
		asset := &CompactAssets{
			AuthTokens: testCase.authTokens,
		}

		actual := asset.HasAuthTokens()
		if actual != testCase.expected {
			t.Errorf("Expected HasAuthTokens to be %v, but was %v", testCase.expected, actual)
		}
	}
}

func TestHasTLSBootstrapToken(t *testing.T) {
	testCases := []struct {
		tlsBootstrapToken string
		expected          bool
	}{
		// Without TLS bootstrap token
		{
			tlsBootstrapToken: "",
			expected:          false,
		},

		// With TLS bootstrap token
		{
			tlsBootstrapToken: "contents",
			expected:          true,
		},
	}

	for _, testCase := range testCases {
		asset := &CompactAssets{
			TLSBootstrapToken: testCase.tlsBootstrapToken,
		}

		actual := asset.HasTLSBootstrapToken()
		if actual != testCase.expected {
			t.Errorf("Expected HasTLSBootstrapToken to be %v, but was %v", testCase.expected, actual)
		}
	}
}
