package model

import (
	"fmt"
	"strings"
	"testing"
)

func TestNodePoolConfig(t *testing.T) {
	mainClusterYaml := `
clusterName: mycluster
keyName: test-key-name
region: ap-northeast-1
availabilityZone: ap-northeast-1a
apiEndpoints:
- name: public
  dnsName: kubeawstest.example.com
  loadBalancer:
    hostedZone:
      id: hostedzone-xxxx
sshAuthorizedKeys:
- mydummysshpublickey
kmsKeyArn: arn:aws:kms:ap-northeast-1:xxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx
s3URI: s3//bucket/emptyDir
worker:
  nodePools:
  - name: foo
`

	parseErrorCases := []struct {
		context              string
		configYaml           string
		expectedErrorMessage string
	}{
		{
			context: "WithInvalidTaint",
			configYaml: mainClusterYaml + `
    taints:
      - key: foo
        value: bar
        effect: UnknownEffect
`,
			expectedErrorMessage: "invalid taint effect: UnknownEffect",
		},
		{
			context: "WithVpcIdAndVPCCIDRSpecified",
			configYaml: mainClusterYaml + `
    vpcId: vpc-1a2b3c4d
    # vpcCIDR (10.1.0.0/16) does not contain instanceCIDR (10.0.1.0/24)
    vpcCIDR: "10.1.0.0/16"
`,
		},
		{
			context: "WithSpotFleetWithInvalidRootVolumeType",
			configYaml: mainClusterYaml + `
    spotFleet:
      targetCapacity: 10
      launchSpecifications:
      - weightedCapacity: 1
        instanceType: c4.large
        rootVolume:
          type: foo
`,
		},
		{
			context: "WithSpotFleetWithInvalidRootVolumeIOPS",
			configYaml: mainClusterYaml + `
    spotFleet:
      targetCapacity: 10
      launchSpecifications:
      - weightedCapacity: 1
        instanceType: c4.large
        rootVolume:
          type: io1
          # must be 100~20000
          iops: 50
`,
		},
		{
			context: "WithSpotFleetWithInvalidRootVolumeTypeAndIOPSCombination",
			configYaml: mainClusterYaml + `
    spotFleet:
      targetCapacity: 10
      launchSpecifications:
      - weightedCapacity: 1
        instanceType: c4.large
        rootVolume:
          type: gp2
          iops: 1000
`,
		},
		{
			context: "WithWorkerSecurityGroupIds",
			configYaml: mainClusterYaml + `
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
			configYaml: mainClusterYaml + `
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
			configYaml: mainClusterYaml + `
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
			configYaml: mainClusterYaml + `
    kmsKeyArn: "arn:aws:kms:us-west-1:xxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx"
`,
			expectedErrorMessage: "although you can't customize `kmsKeyArn` per node pool but you did specify",
		},
		{
			context: "WithRegion",
			configYaml: mainClusterYaml + `
    region: ap-northeast-1"
`,
			expectedErrorMessage: "although you can't customize `region` per node pool but you did specify",
		},
	}

	for _, invalidCase := range parseErrorCases {
		t.Run(invalidCase.context, func(t *testing.T) {
			configBytes := invalidCase.configYaml
			mainConf, err := ConfigFromBytes([]byte(configBytes))
			if err != nil {
				errorMsg := fmt.Sprintf("%v", err)
				if !strings.Contains(errorMsg, invalidCase.expectedErrorMessage) {
					t.Errorf(`expected "%s" to be contained in the errror message : %s`, invalidCase.expectedErrorMessage, errorMsg)
					t.FailNow()
				}
				return
			}

			_, err = NodePoolCompile(mainConf.NodePools[0], mainConf)
			if err == nil {
				t.Errorf("expected to fail parsing config %s: %v", configBytes, mainConf)
				t.FailNow()
			}

			errorMsg := fmt.Sprintf("%v", err)
			if !strings.Contains(errorMsg, invalidCase.expectedErrorMessage) {
				t.Errorf(`expected "%s" to be contained in the errror message : %s`, invalidCase.expectedErrorMessage, errorMsg)
			}
		})
	}
}
