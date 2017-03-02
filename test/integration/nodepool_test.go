package integration

import (
	"fmt"
	cfg "github.com/coreos/kube-aws/core/controlplane/config"
	"github.com/coreos/kube-aws/core/nodepool/config"
	"strings"
	"testing"
)

const nodepoolInsufficientConfigYaml = `clusterName: mycluster
nodePoolName: myculster-pool1
externalDNSName: test.staging.core-os.net
keyName: test-key-name
`

type nodePoolSettings struct {
	kubeAwsSettings
	nodePoolName        string
	nodePoolClusterYaml string
}

func newNodePoolSettingsFromEnv(t *testing.T) nodePoolSettings {
	env := testEnv{t: t}

	kubeAwsSettings := newKubeAwsSettingsFromEnv(t)

	if useRealAWS() {
		nodePoolName := env.get("KUBE_AWS_NODE_POOL_NAME")
		yaml := fmt.Sprintf(`clusterName: %s
name: %s
externalDNSName: "%s"
keyName: "%s"
`, kubeAwsSettings.clusterName, nodePoolName, kubeAwsSettings.externalDNSName, kubeAwsSettings.keyName)
		return nodePoolSettings{
			kubeAwsSettings:     kubeAwsSettings,
			nodePoolName:        nodePoolName,
			nodePoolClusterYaml: yaml,
		}
	} else {
		return nodePoolSettings{
			kubeAwsSettings:     kubeAwsSettings,
			nodePoolClusterYaml: nodepoolInsufficientConfigYaml,
		}
	}
}

type NodePoolConfigTester func(c *config.ProvidedConfig, t *testing.T)

func TestNodePoolConfig(t *testing.T) {
	settings := newNodePoolSettingsFromEnv(t)
	minimalValidConfigYaml := settings.nodePoolClusterYaml + `
availabilityZone: us-west-1c
dnsServiceIP: "10.3.0.10"
etcdEndpoints: "10.0.0.1"
`

	mainClusterYaml := `
region: ap-northeast-1
availabilityZone: ap-northeast-1a
externalDNSName: kubeawstest.example.com
sshAuthorizedKeys:
- mydummysshpublickey
kmsKeyArn: mykmskeyarn
`
	mainCluster, err := cfg.ClusterFromBytes([]byte(mainClusterYaml))
	if err != nil {
		t.Errorf("failed to read the test cluster : %v", err)
		t.FailNow()
	}
	mainConfig, err := mainCluster.Config()
	if err != nil {
		t.Errorf("failed to generate the config for the default cluster : %v", err)
		t.FailNow()
	}

	parseErrorCases := []struct {
		context              string
		configYaml           string
		expectedErrorMessage string
	}{
		{
			context: "WithInvalidTaint",
			configYaml: minimalValidConfigYaml + `
taints:
  - key: foo
    value: bar
    effect: UnknownEffect
`,
			expectedErrorMessage: "Effect must be NoSchdule or PreferNoSchedule, but was UnknownEffect",
		},
		{
			context: "WithVpcIdAndVPCCIDRSpecified",
			configYaml: minimalValidConfigYaml + `
vpcId: vpc-1a2b3c4d
# vpcCIDR (10.1.0.0/16) does not contain instanceCIDR (10.0.1.0/24)
vpcCIDR: "10.1.0.0/16"
`,
		},
		{
			context: "WithSpotFleetWithInvalidRootVolumeType",
			configYaml: minimalValidConfigYaml + `
spotFleet:
  targetCapacity: 10
  launchSpecifications:
  - weightedCapacity: 1
    instanceType: c4.large
    rootVolumeType: foo
`,
		},
		{
			context: "WithSpotFleetWithInvalidRootVolumeIOPS",
			configYaml: minimalValidConfigYaml + `
spotFleet:
  targetCapacity: 10
  launchSpecifications:
  - weightedCapacity: 1
    instanceType: c4.large
    rootVolumeType: io1
    # must be 100~2000
    rootVolumeIOPS: 50
`,
		},
		{
			context: "WithSpotFleetWithInvalidRootVolumeTypeAndIOPSCombination",
			configYaml: minimalValidConfigYaml + `
spotFleet:
  targetCapacity: 10
  launchSpecifications:
  - weightedCapacity: 1
    instanceType: c4.large
    rootVolumeType: gp2
    rootVolumeIOPS: 1000
`,
		},
		{
			context: "WithWorkerSecurityGroupIds",
			configYaml: minimalValidConfigYaml + `
securityGroupIds:
  - sg-12345678
  - sg-abcdefab
  - sg-23456789
  - sg-bcdefabc
  - sg-34567890
`,
			expectedErrorMessage: "number of user provided security groups must be less than or equal to 4 but was 5",
		},
		{
			context: "WithWorkerAndLBSecurityGroupIds",
			configYaml: minimalValidConfigYaml + `
securityGroupIds:
  - sg-12345678
  - sg-abcdefab
  - sg-23456789
loadBalancer:
  enabled: true
  securityGroupIds:
    - sg-bcdefabc
    - sg-34567890
`,
			expectedErrorMessage: "number of user provided security groups must be less than or equal to 4 but was 5",
		},
		{
			context: "WithWorkerAndALBSecurityGroupIds",
			configYaml: minimalValidConfigYaml + `
securityGroupIds:
  - sg-12345678
  - sg-abcdefab
  - sg-23456789
targetGroup:
  enabled: true
  securityGroupIds:
    - sg-bcdefabc
    - sg-34567890
`,
			expectedErrorMessage: "number of user provided security groups must be less than or equal to 4 but was 5",
		},
		{
			context: "WithKmsKeyArn",
			configYaml: minimalValidConfigYaml + `
kmsKeyArn: "arn:aws:kms:us-west-1:xxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx"
`,
			expectedErrorMessage: "although you can't customize `kmsKeyArn` per node pool but you did specify",
		},
		{
			context: "WithRegion",
			configYaml: minimalValidConfigYaml + `
region: ap-northeast-1"
`,
			expectedErrorMessage: "although you can't customize `region` per node pool but you did specify",
		},
	}

	for _, invalidCase := range parseErrorCases {
		t.Run(invalidCase.context, func(t *testing.T) {
			configBytes := invalidCase.configYaml
			providedConfig, err := config.ClusterFromBytes([]byte(configBytes), mainConfig)
			if err == nil {
				t.Errorf("expected to fail parsing config %s: %v", configBytes, providedConfig)
				t.FailNow()
			}

			errorMsg := fmt.Sprintf("%v", err)
			if !strings.Contains(errorMsg, invalidCase.expectedErrorMessage) {
				t.Errorf(`expected "%s" to be contained in the errror message : %s`, invalidCase.expectedErrorMessage, errorMsg)
			}
		})
	}
}
