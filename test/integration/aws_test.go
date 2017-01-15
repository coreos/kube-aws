package integration

import (
	"fmt"
	"github.com/coreos/kube-aws/nodepool/cluster"
	"github.com/coreos/kube-aws/nodepool/config"
	"github.com/coreos/kube-aws/test/helper"
	"os"
	"testing"
)

var yaml string

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

type integrationSettings struct {
	nodePoolName    string
	externalDNSName string
	keyName         string
	kmsKeyArn       string
	region          string
}

func newIntegrationSettings(t *testing.T) integrationSettings {
	env := testEnv{t: t}

	nodePoolName := env.get("KUBE_AWS_NODE_POOL_NAME")
	return integrationSettings{
		nodePoolName:    nodePoolName,
		externalDNSName: fmt.Sprintf("%s.%s", nodePoolName, env.get("KUBE_AWS_DOMAIN")),
		keyName:         env.get("KUBE_AWS_KEY_NAME"),
		kmsKeyArn:       env.get("KUBE_AWS_KMS_KEY_ARN"),
		region:          env.get("KUBE_AWS_REGION"),
	}
}

func insufficientConfigYamlFromEnv(t *testing.T) string {
	settings := newIntegrationSettings(t)

	yaml = fmt.Sprintf(`clusterName: mycluster
nodePoolName: %s
externalDNSName: "%s"
keyName: "%s"
kmsKeyArn: "%s"
region: "%s"
`,
		settings.nodePoolName,
		settings.externalDNSName,
		settings.keyName,
		settings.kmsKeyArn,
		settings.region,
	)

	return yaml
}

// Integration testing with real AWS services including S3, KMS, CloudFormation
func TestAwsIntegration(t *testing.T) {
	if os.Getenv("KUBE_AWS_INTEGRATION_TEST") == "" {
		t.Skipf("`export KUBE_AWS_INTEGRATION_TEST=1` is required to run integration tests. Skipping.")
		t.SkipNow()
	}

	yaml := insufficientConfigYamlFromEnv(t)

	minimalValidConfigYaml := yaml + `
availabilityZone: us-west-1c
dnsServiceIP: "10.3.0.10"
etcdEndpoints: "10.0.0.1"
`
	validCases := []struct {
		context    string
		configYaml string
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
		},
		{
			context:    "WithMinimalValidConfig",
			configYaml: minimalValidConfigYaml,
		},
		{
			context: "WithVpcIdSpecified",
			configYaml: minimalValidConfigYaml + `
vpcId: vpc-1a2b3c4d
`,
		},
		{
			context: "WithVpcIdAndRouteTableIdSpecified",
			configYaml: minimalValidConfigYaml + `
vpcId: vpc-1a2b3c4d
routeTableId: rtb-1a2b3c4d
`,
		},
		{
			context: "WithEtcdNodesWithCustomEBSVolumes",
			configYaml: minimalValidConfigYaml + `
vpcId: vpc-1a2b3c4d
routeTableId: rtb-1a2b3c4d
etcdCount: 2
etcdRootVolumeSize: 101
etcdRootVolumeType: io1
etcdRootVolumeIOPS: 102
etcdDataVolumeSize: 103
etcdDataVolumeType: io1
etcdDataVolumeIOPS: 104
`,
		},
		{
			context: "WithSpotFleetEnabled",
			configYaml: minimalValidConfigYaml + `
worker:
  spotFleet:
    targetCapacity: 10
`,
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
      instanceType: t2.medium
    - weightedCapacity: 2
      instanceType: m3.large
      rootVolumeSize: 100
`,
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
      instanceType: t2.medium
    - weightedCapacity: 2
      instanceType: m3.large
      rootVolumeIOPS: 500
`,
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
		},
	}

	for _, validCase := range validCases {
		t.Run(validCase.context, func(t *testing.T) {
			configBytes := validCase.configYaml
			providedConfig, err := config.ClusterFromBytes([]byte(configBytes))
			if err != nil {
				t.Errorf("failed to parse config %s: %v", configBytes, err)
				t.FailNow()
			}

			helper.WithDummyCredentials(func(dummyTlsAssetsDir string) {
				var stackTemplateOptions = config.StackTemplateOptions{
					TLSAssetsDir:          dummyTlsAssetsDir,
					WorkerTmplFile:        "../../config/templates/cloud-config-worker",
					StackTemplateTmplFile: "../../nodepool/config/templates/stack-template.json",
				}

				t.Run("ValidateUserData", func(t *testing.T) {
					if err := providedConfig.ValidateUserData(stackTemplateOptions); err != nil {
						t.Errorf("failed to validate user data: %v", err)
					}
				})

				t.Run("RenderStackTemplate", func(t *testing.T) {
					if _, err := providedConfig.RenderStackTemplate(stackTemplateOptions, false); err != nil {
						t.Errorf("failed to render stack template: %v", err)
					}
				})

				t.Run("ValidateStack", func(t *testing.T) {
					stackTemplate, err := providedConfig.RenderStackTemplate(stackTemplateOptions, false)

					if err != nil {
						t.Errorf("failed to render stack template: %v", err)
					}

					cluster := cluster.New(providedConfig, true)
					s3URI, exists := os.LookupEnv("KUBE_AWS_S3_DIR_URI")

					if !exists {
						t.Errorf("failed to obtain value for KUBE_AWS_S3_DIR_URI")
						t.FailNow()
					}

					report, err := cluster.ValidateStack(string(stackTemplate), s3URI)

					if err != nil {
						t.Errorf("failed to validate stack: %s", report)
					}
				})
			})
		})
	}
}
