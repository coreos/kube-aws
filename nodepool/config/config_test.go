package config

import (
	"fmt"
	"github.com/aws/aws-sdk-go/service/kms"
	cfg "github.com/coreos/kube-aws/config"
	model "github.com/coreos/kube-aws/model"
	"github.com/coreos/kube-aws/test/helper"
	"reflect"
	"strings"
	"testing"
)

type dummyEncryptService struct{}

func (d dummyEncryptService) Encrypt(input *kms.EncryptInput) (*kms.EncryptOutput, error) {
	output := kms.EncryptOutput{
		CiphertextBlob: input.Plaintext,
	}
	return &output, nil
}

const insufficientConfigYaml = `clusterName: mycluster
nodePoolName: myculster-pool1
externalDNSName: test.staging.core-os.net
keyName: test-key-name
kmsKeyArn: "arn:aws:kms:us-west-1:xxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx"
region: us-west-1
`

const availabilityZoneConfig = `
availabilityZone: us-west-1c
`

type ConfigTester func(c *ProvidedConfig, t *testing.T)

func TestConfig(t *testing.T) {
	minimalValidConfigYaml := insufficientConfigYaml + `
availabilityZone: us-west-1c
dnsServiceIP: "10.3.0.10"
etcdEndpoints: "10.0.0.1"
`
	hasDefaultLaunchSpecifications := func(c *ProvidedConfig, t *testing.T) {
		expected := []model.LaunchSpecification{
			{
				WeightedCapacity: 1,
				InstanceType:     "m3.medium",
				RootVolumeSize:   30,
				RootVolumeIOPS:   0,
				RootVolumeType:   "gp2",
			},
			{
				WeightedCapacity: 2,
				InstanceType:     "m3.large",
				RootVolumeSize:   60,
				RootVolumeIOPS:   0,
				RootVolumeType:   "gp2",
			},
			{
				WeightedCapacity: 2,
				InstanceType:     "m4.large",
				RootVolumeSize:   60,
				RootVolumeIOPS:   0,
				RootVolumeType:   "gp2",
			},
		}
		actual := c.Worker.SpotFleet.LaunchSpecifications
		if !reflect.DeepEqual(expected, actual) {
			t.Errorf(
				"LaunchSpecifications didn't match: expected=%v actual=%v",
				expected,
				actual,
			)
		}
	}

	hasDefaultExperimentalFeatures := func(c *ProvidedConfig, t *testing.T) {
		expected := cfg.Experimental{
			AuditLog: cfg.AuditLog{
				Enabled: false,
				MaxAge:  30,
				LogPath: "/dev/stdout",
			},
			AwsEnvironment: cfg.AwsEnvironment{
				Enabled: false,
			},
			EphemeralImageStorage: cfg.EphemeralImageStorage{
				Enabled:    false,
				Disk:       "xvdb",
				Filesystem: "xfs",
			},
			LoadBalancer: cfg.LoadBalancer{
				Enabled: false,
			},
			NodeDrainer: cfg.NodeDrainer{
				Enabled: false,
			},
			NodeLabel: cfg.NodeLabel{
				Enabled: false,
			},
			Taints: []cfg.Taint{},
			WaitSignal: cfg.WaitSignal{
				Enabled:      false,
				MaxBatchSize: 1,
			},
		}

		actual := c.Experimental

		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("experimental settings didn't match :\nexpected=%v\nactual=%v", expected, actual)
		}
	}

	validCases := []struct {
		context              string
		configYaml           string
		assertProvidedConfig []ConfigTester
	}{
		{
			context: "WithExperimentalFeatures",
			configYaml: minimalValidConfigYaml + `
experimental:
  awsEnvironment:
    enabled: true
    environment:
      CFNSTACK: '{ "Ref" : "AWS::StackId" }'
  ephemeralImageStorage:
    enabled: true
  loadBalancer:
    enabled: true
    names:
      - manuallymanagedlb
    securityGroupIds:
      - sg-12345678
  nodeDrainer:
    enabled: true
  nodeLabel:
    enabled: true
  taints:
    - key: reservation
      value: spot
      effect: NoSchedule
  waitSignal:
    enabled: true
`,
			assertProvidedConfig: []ConfigTester{
				hasDefaultLaunchSpecifications,
				func(c *ProvidedConfig, t *testing.T) {
					expected := cfg.Experimental{
						AuditLog: cfg.AuditLog{
							Enabled: false,
							MaxAge:  30,
							LogPath: "/dev/stdout",
						},
						AwsEnvironment: cfg.AwsEnvironment{
							Enabled: true,
							Environment: map[string]string{
								"CFNSTACK": `{ "Ref" : "AWS::StackId" }`,
							},
						},
						EphemeralImageStorage: cfg.EphemeralImageStorage{
							Enabled:    true,
							Disk:       "xvdb",
							Filesystem: "xfs",
						},
						LoadBalancer: cfg.LoadBalancer{
							Enabled:          true,
							Names:            []string{"manuallymanagedlb"},
							SecurityGroupIds: []string{"sg-12345678"},
						},
						NodeDrainer: cfg.NodeDrainer{
							Enabled: true,
						},
						NodeLabel: cfg.NodeLabel{
							Enabled: true,
						},
						Taints: []cfg.Taint{
							{Key: "reservation", Value: "spot", Effect: "NoSchedule"},
						},
						WaitSignal: cfg.WaitSignal{
							Enabled:      true,
							MaxBatchSize: 1,
						},
					}

					actual := c.Experimental

					if !reflect.DeepEqual(expected, actual) {
						t.Errorf("experimental settings didn't match : expected=%v actual=%v", expected, actual)
					}
				},
			},
		},
		{
			context:    "WithMinimalValidConfig",
			configYaml: minimalValidConfigYaml,
			assertProvidedConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				hasDefaultLaunchSpecifications,
			}},
		{
			context: "WithVpcIdSpecified",
			configYaml: minimalValidConfigYaml + `
vpcId: vpc-1a2b3c4d
`,
			assertProvidedConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				hasDefaultLaunchSpecifications,
			},
		},
		{
			context: "WithVpcIdAndRouteTableIdSpecified",
			configYaml: minimalValidConfigYaml + `
vpcId: vpc-1a2b3c4d
routeTableId: rtb-1a2b3c4d
`,
			assertProvidedConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				hasDefaultLaunchSpecifications,
			},
		},
		{
			context: "WithSpotFleetEnabled",
			configYaml: minimalValidConfigYaml + `
worker:
  spotFleet:
    targetCapacity: 10
`,
			assertProvidedConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				hasDefaultLaunchSpecifications,
			},
		},
		{
			context: "WithSpotFleetWithCustomGp2RootVolumeSettings",
			configYaml: minimalValidConfigYaml + `
worker:
  spotFleet:
    targetCapacity: 10
    unitRootVolumeSize: 40
    launchSpecifications:
    - weightedCapacity: 1
      instanceType: m3.medium
    - weightedCapacity: 2
      instanceType: m3.large
      rootVolumeSize: 100
`,
			assertProvidedConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				func(c *ProvidedConfig, t *testing.T) {
					expected := []model.LaunchSpecification{
						{
							WeightedCapacity: 1,
							InstanceType:     "m3.medium",
							// RootVolumeSize was not specified in the configYaml but should default to workerRootVolumeSize * weightedCapacity hence:
							RootVolumeSize: 40,
							RootVolumeIOPS: 0,
							// RootVolumeType was not specified in the configYaml but should default to:
							RootVolumeType: "gp2",
						},
						{
							WeightedCapacity: 2,
							InstanceType:     "m3.large",
							RootVolumeSize:   100,
							RootVolumeIOPS:   0,
							RootVolumeType:   "gp2",
						},
					}
					actual := c.Worker.SpotFleet.LaunchSpecifications
					if !reflect.DeepEqual(expected, actual) {
						t.Errorf(
							"LaunchSpecifications didn't match: expected=%v actual=%v",
							expected,
							actual,
						)
					}
				},
			},
		},
		{
			context: "WithSpotFleetWithCustomIo1RootVolumeSettings",
			configYaml: minimalValidConfigYaml + `
worker:
  spotFleet:
    targetCapacity: 10
    rootVolumeType: io1
    unitRootVolumeSize: 40
    unitRootVolumeIOPS: 100
    launchSpecifications:
    - weightedCapacity: 1
      instanceType: m3.medium
    - weightedCapacity: 2
      instanceType: m3.large
      rootVolumeIOPS: 500
`,
			assertProvidedConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				func(c *ProvidedConfig, t *testing.T) {
					expected := []model.LaunchSpecification{
						{
							WeightedCapacity: 1,
							InstanceType:     "m3.medium",
							// RootVolumeSize was not specified in the configYaml but should default to workerRootVolumeSize * weightedCapacity hence:
							RootVolumeSize: 40,
							// RootVolumeIOPS was not specified in the configYaml but should default to workerRootVolumeIOPS * weightedCapacity hence:
							RootVolumeIOPS: 100,
							// RootVolumeType was not specified in the configYaml but should default to:
							RootVolumeType: "io1",
						},
						{
							WeightedCapacity: 2,
							InstanceType:     "m3.large",
							RootVolumeSize:   80,
							RootVolumeIOPS:   500,
							// RootVolumeType was not specified in the configYaml but should default to:
							RootVolumeType: "io1",
						},
					}
					actual := c.Worker.SpotFleet.LaunchSpecifications
					if !reflect.DeepEqual(expected, actual) {
						t.Errorf(
							"LaunchSpecifications didn't match: expected=%v actual=%v",
							expected,
							actual,
						)
					}
				},
			},
		},
		{
			context: "WithWorkerSecurityGroupIds",
			configYaml: minimalValidConfigYaml + `
workerSecurityGroupIds:
  - sg-12345678
  - sg-abcdefab
  - sg-23456789
  - sg-bcdefabc
`,
			assertProvidedConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				hasDefaultLaunchSpecifications,
				func(c *ProvidedConfig, t *testing.T) {
					expectedWorkerSecurityGroupIds := []string{
						`sg-12345678`, `sg-abcdefab`, `sg-23456789`, `sg-bcdefabc`,
					}
					if !reflect.DeepEqual(c.WorkerSecurityGroupIds, expectedWorkerSecurityGroupIds) {
						t.Errorf("WorkerSecurityGroupIds didn't match: expected=%v actual=%v", expectedWorkerSecurityGroupIds, c.WorkerSecurityGroupIds)
					}

					expectedWorkerSecurityGroupRefs := []string{
						`"sg-12345678"`, `"sg-abcdefab"`, `"sg-23456789"`, `"sg-bcdefabc"`,
					}
					if !reflect.DeepEqual(c.WorkerDeploymentSettings().WorkerSecurityGroupRefs(), expectedWorkerSecurityGroupRefs) {
						t.Errorf("WorkerSecurityGroupRefs didn't match: expected=%v actual=%v", expectedWorkerSecurityGroupRefs, c.WorkerDeploymentSettings().WorkerSecurityGroupRefs())
					}
				},
			},
		},
		{
			context: "WithWorkerAndLBSecurityGroupIds",
			configYaml: minimalValidConfigYaml + `
workerSecurityGroupIds:
  - sg-12345678
  - sg-abcdefab
experimental:
  loadBalancer:
    enabled: true
    securityGroupIds:
      - sg-23456789
      - sg-bcdefabc
`,
			assertProvidedConfig: []ConfigTester{
				hasDefaultLaunchSpecifications,
				func(c *ProvidedConfig, t *testing.T) {
					expectedWorkerSecurityGroupIds := []string{
						`sg-12345678`, `sg-abcdefab`,
					}
					if !reflect.DeepEqual(c.WorkerSecurityGroupIds, expectedWorkerSecurityGroupIds) {
						t.Errorf("WorkerSecurityGroupIds didn't match: expected=%v actual=%v", expectedWorkerSecurityGroupIds, c.WorkerSecurityGroupIds)
					}

					expectedLBSecurityGroupIds := []string{
						`sg-23456789`, `sg-bcdefabc`,
					}
					if !reflect.DeepEqual(c.Experimental.LoadBalancer.SecurityGroupIds, expectedLBSecurityGroupIds) {
						t.Errorf("LBSecurityGroupIds didn't match: expected=%v actual=%v", expectedLBSecurityGroupIds, c.Experimental.LoadBalancer.SecurityGroupIds)
					}

					expectedWorkerSecurityGroupRefs := []string{
						`"sg-23456789"`, `"sg-bcdefabc"`, `"sg-12345678"`, `"sg-abcdefab"`,
					}
					if !reflect.DeepEqual(c.WorkerDeploymentSettings().WorkerSecurityGroupRefs(), expectedWorkerSecurityGroupRefs) {
						t.Errorf("WorkerSecurityGroupRefs didn't match: expected=%v actual=%v", expectedWorkerSecurityGroupRefs, c.WorkerDeploymentSettings().WorkerSecurityGroupRefs())
					}
				},
			},
		},
	}

	for _, validCase := range validCases {
		t.Run(validCase.context, func(t *testing.T) {
			configBytes := validCase.configYaml
			providedConfig, err := ClusterFromBytes([]byte(configBytes))
			if err != nil {
				t.Errorf("failed to parse config %s: %v", configBytes, err)
				t.FailNow()
			}
			providedConfig.providedEncryptService = dummyEncryptService{}

			t.Run("AssertProvidedConfig", func(t *testing.T) {
				for _, assertion := range validCase.assertProvidedConfig {
					assertion(providedConfig, t)
				}
			})

			helper.WithDummyCredentials(func(dummyTlsAssetsDir string) {
				var stackTemplateOptions = StackTemplateOptions{
					TLSAssetsDir:          dummyTlsAssetsDir,
					WorkerTmplFile:        "../../config/templates/cloud-config-worker",
					StackTemplateTmplFile: "templates/stack-template.json",
				}

				t.Run("ValidateUserData", func(t *testing.T) {
					if err := providedConfig.ValidateUserData(stackTemplateOptions); err != nil {
						t.Errorf("failed to validate user data: %v", err)
					}
				})

				t.Run("RenderStackTemplate", func(t *testing.T) {
					if _, err := providedConfig.RenderStackTemplate(stackTemplateOptions); err != nil {
						t.Errorf("failed to render stack template: %v", err)
					}
				})
			})
		})
	}

	parseErrorCases := []struct {
		context              string
		configYaml           string
		expectedErrorMessage string
	}{
		{
			context: "WithInvalidTaint",
			configYaml: minimalValidConfigYaml + `
experimental:
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
worker:
  spotFleet:
    targetCapacity: 10
    launchSpecifications:
    - weightedCapacity: 1
      instanceType: m3.medium
      rootVolumeType: foo
`,
		},
		{
			context: "WithSpotFleetWithInvalidRootVolumeIOPS",
			configYaml: minimalValidConfigYaml + `
worker:
  spotFleet:
    targetCapacity: 10
    launchSpecifications:
    - weightedCapacity: 1
      instanceType: m3.medium
      rootVolumeType: io1
      # must be 100~2000
      rootVolumeIOPS: 50
`,
		},
		{
			context: "WithSpotFleetWithInvalidRootVolumeTypeAndIOPSCombination",
			configYaml: minimalValidConfigYaml + `
worker:
  spotFleet:
    targetCapacity: 10
    launchSpecifications:
    - weightedCapacity: 1
      instanceType: m3.medium
      rootVolumeType: gp2
      rootVolumeIOPS: 1000
`,
		},
		{
			context: "WithWorkerSecurityGroupIds",
			configYaml: minimalValidConfigYaml + `
workerSecurityGroupIds:
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
workerSecurityGroupIds:
  - sg-12345678
  - sg-abcdefab
  - sg-23456789
experimental:
  loadBalancer:
    enabled: true
    securityGroupIds:
      - sg-bcdefabc
      - sg-34567890
`,
			expectedErrorMessage: "number of user provided security groups must be less than or equal to 4 but was 5",
		},
	}

	for _, invalidCase := range parseErrorCases {
		t.Run(invalidCase.context, func(t *testing.T) {
			configBytes := invalidCase.configYaml
			providedConfig, err := ClusterFromBytes([]byte(configBytes))
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
