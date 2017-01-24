package integration

import (
	"fmt"
	cfg "github.com/coreos/kube-aws/config"
	"github.com/coreos/kube-aws/model"
	"github.com/coreos/kube-aws/nodepool/cluster"
	"github.com/coreos/kube-aws/nodepool/config"
	"github.com/coreos/kube-aws/test/helper"
	"os"
	"reflect"
	"strings"
	"testing"
)

const nodepoolInsufficientConfigYaml = `clusterName: mycluster
nodePoolName: myculster-pool1
externalDNSName: test.staging.core-os.net
keyName: test-key-name
kmsKeyArn: "arn:aws:kms:us-west-1:xxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx"
region: us-west-1
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
nodePoolName: %s
externalDNSName: "%s"
keyName: "%s"
kmsKeyArn: "%s"
region: "%s"
`, kubeAwsSettings.clusterName, nodePoolName, kubeAwsSettings.externalDNSName, kubeAwsSettings.keyName, kubeAwsSettings.kmsKeyArn, kubeAwsSettings.region)
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
	hasDefaultLaunchSpecifications := func(c *config.ProvidedConfig, t *testing.T) {
		expected := []model.LaunchSpecification{
			{
				WeightedCapacity: 1,
				InstanceType:     "c4.large",
				RootVolumeSize:   30,
				RootVolumeIOPS:   0,
				RootVolumeType:   "gp2",
			},
			{
				WeightedCapacity: 2,
				InstanceType:     "c4.xlarge",
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

	hasDefaultExperimentalFeatures := func(c *config.ProvidedConfig, t *testing.T) {
		expected := cfg.Experimental{
			AuditLog: cfg.AuditLog{
				Enabled: false,
				MaxAge:  30,
				LogPath: "/dev/stdout",
			},
			AwsEnvironment: cfg.AwsEnvironment{
				Enabled: false,
			},
			AwsNodeLabels: cfg.AwsNodeLabels{
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
			NodeLabels: cfg.NodeLabels{},
			Taints:     []cfg.Taint{},
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
		assertProvidedConfig []NodePoolConfigTester
	}{
		{
			context: "WithExperimentalFeatures",
			configYaml: minimalValidConfigYaml + `
experimental:
  awsEnvironment:
    enabled: true
    environment:
      CFNSTACK: '{ "Ref" : "AWS::StackId" }'
  awsNodeLabels:
    enabled: true
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
  nodeLabels:
    kube-aws.coreos.com/role: worker
  taints:
    - key: reservation
      value: spot
      effect: NoSchedule
  waitSignal:
    enabled: true
`,
			assertProvidedConfig: []NodePoolConfigTester{
				hasDefaultLaunchSpecifications,
				func(c *config.ProvidedConfig, t *testing.T) {
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
						AwsNodeLabels: cfg.AwsNodeLabels{
							Enabled: true,
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
						NodeLabels: cfg.NodeLabels{
							"kube-aws.coreos.com/role": "worker",
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
			assertProvidedConfig: []NodePoolConfigTester{
				hasDefaultExperimentalFeatures,
				hasDefaultLaunchSpecifications,
			}},
		{
			context: "WithVpcIdSpecified",
			configYaml: minimalValidConfigYaml + `
vpcId: vpc-1a2b3c4d
`,
			assertProvidedConfig: []NodePoolConfigTester{
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
			assertProvidedConfig: []NodePoolConfigTester{
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
			assertProvidedConfig: []NodePoolConfigTester{
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
      instanceType: c4.large
    - weightedCapacity: 2
      instanceType: c4.xlarge
      rootVolumeSize: 100
`,
			assertProvidedConfig: []NodePoolConfigTester{
				hasDefaultExperimentalFeatures,
				func(c *config.ProvidedConfig, t *testing.T) {
					expected := []model.LaunchSpecification{
						{
							WeightedCapacity: 1,
							InstanceType:     "c4.large",
							// RootVolumeSize was not specified in the configYaml but should default to workerRootVolumeSize * weightedCapacity hence:
							RootVolumeSize: 40,
							RootVolumeIOPS: 0,
							// RootVolumeType was not specified in the configYaml but should default to:
							RootVolumeType: "gp2",
						},
						{
							WeightedCapacity: 2,
							InstanceType:     "c4.xlarge",
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
			context: "WithSpotFleetWithCustomInstanceTypes",
			configYaml: minimalValidConfigYaml + `
worker:
  spotFleet:
    targetCapacity: 10
    unitRootVolumeSize: 40
    launchSpecifications:
    - weightedCapacity: 1
      instanceType: m4.large
    - weightedCapacity: 2
      instanceType: m4.xlarge
`,
			assertProvidedConfig: []NodePoolConfigTester{
				hasDefaultExperimentalFeatures,
				func(c *config.ProvidedConfig, t *testing.T) {
					expected := []model.LaunchSpecification{
						{
							WeightedCapacity: 1,
							InstanceType:     "m4.large",
							RootVolumeSize:   40,
							RootVolumeIOPS:   0,
							// RootVolumeType was not specified in the configYaml but should default to:
							RootVolumeType: "gp2",
						},
						{
							WeightedCapacity: 2,
							InstanceType:     "m4.xlarge",
							RootVolumeSize:   80,
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
      instanceType: c4.large
    - weightedCapacity: 2
      instanceType: c4.xlarge
      rootVolumeIOPS: 500
`,
			assertProvidedConfig: []NodePoolConfigTester{
				hasDefaultExperimentalFeatures,
				func(c *config.ProvidedConfig, t *testing.T) {
					expected := []model.LaunchSpecification{
						{
							WeightedCapacity: 1,
							InstanceType:     "c4.large",
							// RootVolumeSize was not specified in the configYaml but should default to workerRootVolumeSize * weightedCapacity hence:
							RootVolumeSize: 40,
							// RootVolumeIOPS was not specified in the configYaml but should default to workerRootVolumeIOPS * weightedCapacity hence:
							RootVolumeIOPS: 100,
							// RootVolumeType was not specified in the configYaml but should default to:
							RootVolumeType: "io1",
						},
						{
							WeightedCapacity: 2,
							InstanceType:     "c4.xlarge",
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
			assertProvidedConfig: []NodePoolConfigTester{
				hasDefaultExperimentalFeatures,
				hasDefaultLaunchSpecifications,
				func(c *config.ProvidedConfig, t *testing.T) {
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
			assertProvidedConfig: []NodePoolConfigTester{
				hasDefaultLaunchSpecifications,
				func(c *config.ProvidedConfig, t *testing.T) {
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

	for _, validCase := range validCases {
		t.Run(validCase.context, func(t *testing.T) {
			configBytes := validCase.configYaml
			providedConfig, err := config.ClusterFromBytesWithEncryptService([]byte(configBytes), mainConfig, settings.encryptService)
			if err != nil {
				t.Errorf("failed to parse config %s: %v", configBytes, err)
				t.FailNow()
			}

			t.Run("AssertProvidedConfig", func(t *testing.T) {
				for _, assertion := range validCase.assertProvidedConfig {
					assertion(providedConfig, t)
				}
			})

			helper.WithDummyCredentials(func(dummyTlsAssetsDir string) {
				s3URI, s3URIExists := os.LookupEnv("KUBE_AWS_S3_DIR_URI")

				var stackTemplateOptions = config.StackTemplateOptions{
					TLSAssetsDir:          dummyTlsAssetsDir,
					WorkerTmplFile:        "../../config/templates/cloud-config-worker",
					StackTemplateTmplFile: "../../nodepool/config/templates/stack-template.json",
					S3URI: s3URI,
				}

				cluster, err := cluster.NewCluster(providedConfig, stackTemplateOptions, false)
				if err != nil {
					t.Errorf("failed to create cluster driver : %v", err)
					t.FailNow()
				}

				t.Run("ValidateUserData", func(t *testing.T) {
					if err := cluster.ValidateUserData(); err != nil {
						t.Errorf("failed to validate user data: %v", err)
					}
				})

				t.Run("RenderStackTemplate", func(t *testing.T) {
					if _, err := cluster.RenderStackTemplateAsString(); err != nil {
						t.Errorf("failed to render stack template: %v", err)
					}
				})

				if !useRealAWS() {
					t.Skipf("`export KUBE_AWS_INTEGRATION_TEST=1` is required to run integration tests. Skipping.")
					t.SkipNow()
				} else {
					t.Run("ValidateStack", func(t *testing.T) {
						if !s3URIExists {
							t.Errorf("failed to obtain value for KUBE_AWS_S3_DIR_URI")
							t.FailNow()
						}

						report, err := cluster.ValidateStack()

						if err != nil {
							t.Errorf("failed to validate stack: %s %v", report, err)
						}
					})
				}
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
			context: "WithSpotFleetWithExperimentalWaitSignal",
			configYaml: minimalValidConfigYaml + `
worker:
  spotFleet:
    targetCapacity: 10
experimental:
  waitSignal:
    enabled: true
`,
			expectedErrorMessage: "The experimental feature `waitSignal` assumes a node pool is managed by an ASG rather than a Spot Fleet.",
		},
		{
			context: "WithSpotFleetWithInvalidRootVolumeType",
			configYaml: minimalValidConfigYaml + `
worker:
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
worker:
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
worker:
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
