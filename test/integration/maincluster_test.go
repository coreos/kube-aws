package integration

import (
	"fmt"
	"github.com/coreos/kube-aws/cfnstack"
	controlplane_config "github.com/coreos/kube-aws/core/controlplane/config"
	"github.com/coreos/kube-aws/core/root"
	"github.com/coreos/kube-aws/core/root/config"
	"github.com/coreos/kube-aws/model"
	"github.com/coreos/kube-aws/test/helper"
	"os"
	"reflect"
	"strings"
	"testing"
)

type ConfigTester func(c *config.Config, t *testing.T)
type ClusterTester func(c root.Cluster, t *testing.T)

// Integration testing with real AWS services including S3, KMS, CloudFormation
func TestMainClusterConfig(t *testing.T) {
	kubeAwsSettings := newKubeAwsSettingsFromEnv(t)

	s3URI, s3URIExists := os.LookupEnv("KUBE_AWS_S3_DIR_URI")

	if !s3URIExists || s3URI == "" {
		s3URI = "s3://examplebucket/exampledir"
		t.Logf(`Falling back s3URI to a stub value "%s" for tests of validating stack templates. No assets will actually be uploaded to S3`, s3URI)
	}

	s3Loc, err := cfnstack.S3URIFromString(s3URI)
	s3Bucket := s3Loc.Bucket()
	s3Dir := s3Loc.PathComponents()[0]

	if err != nil {
		t.Errorf("failed to parse s3 uri: %v", err)
		t.FailNow()
	}

	hasDefaultEtcdSettings := func(c *config.Config, t *testing.T) {
		subnet1 := model.NewPublicSubnet("us-west-1c", "10.0.0.0/24")
		subnet1.Name = "Subnet0"
		expected := controlplane_config.EtcdSettings{
			Etcd: model.Etcd{
				Subnets: []model.Subnet{
					subnet1,
				},
			},
			EtcdCount:               1,
			EtcdInstanceType:        "t2.medium",
			EtcdRootVolumeSize:      30,
			EtcdRootVolumeType:      "gp2",
			EtcdRootVolumeIOPS:      0,
			EtcdDataVolumeSize:      30,
			EtcdDataVolumeType:      "gp2",
			EtcdDataVolumeIOPS:      0,
			EtcdDataVolumeEphemeral: false,
			EtcdTenancy:             "default",
		}
		actual := c.EtcdSettings
		if !reflect.DeepEqual(expected, actual) {
			t.Errorf(
				"EtcdSettings didn't match: expected=%v actual=%v",
				expected,
				actual,
			)
		}
	}

	hasDefaultExperimentalFeatures := func(c *config.Config, t *testing.T) {
		expected := controlplane_config.Experimental{
			Admission: controlplane_config.Admission{
				PodSecurityPolicy: controlplane_config.PodSecurityPolicy{
					Enabled: false,
				},
			},
			AuditLog: controlplane_config.AuditLog{
				Enabled: false,
				MaxAge:  30,
				LogPath: "/dev/stdout",
			},
			Authentication: controlplane_config.Authentication{
				Webhook: controlplane_config.Webhook{
					Enabled:  false,
					CacheTTL: "5m0s",
					Config:   "",
				},
			},
			AwsEnvironment: controlplane_config.AwsEnvironment{
				Enabled: false,
			},
			AwsNodeLabels: controlplane_config.AwsNodeLabels{
				Enabled: false,
			},
			ClusterAutoscalerSupport: controlplane_config.ClusterAutoscalerSupport{
				Enabled: false,
			},
			EphemeralImageStorage: controlplane_config.EphemeralImageStorage{
				Enabled:    false,
				Disk:       "xvdb",
				Filesystem: "xfs",
			},
			Kube2IamSupport: controlplane_config.Kube2IamSupport{
				Enabled: false,
			},
			LoadBalancer: controlplane_config.LoadBalancer{
				Enabled: false,
			},
			NodeDrainer: controlplane_config.NodeDrainer{
				Enabled: false,
			},
			NodeLabels: controlplane_config.NodeLabels{},
			Taints:     []controlplane_config.Taint{},
		}

		actual := c.Experimental

		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("experimental settings didn't match :\nexpected=%v\nactual=%v", expected, actual)
		}

		expected2 := controlplane_config.WaitSignal{
			Enabled:      true,
			MaxBatchSize: 1,
		}
		actual2 := c.WaitSignal
		if !reflect.DeepEqual(expected2, actual2) {
			t.Errorf("waitSignal doesn't match:\nexpected=%v\nactual=%v", expected2, actual2)
		}
	}

	everyPublicSubnetHasRouteToIGW := func(c *config.Config, t *testing.T) {
		for i, s := range c.PublicSubnets() {
			if !s.ManageRouteToInternet() {
				t.Errorf("Public subnet %d should have a route to the IGW but it doesn't: %+v", i, s)
			}
		}
	}

	hasDefaultLaunchSpecifications := func(c *config.Config, t *testing.T) {
		expected := []model.LaunchSpecification{
			{
				WeightedCapacity: 1,
				InstanceType:     "c4.large",
				SpotPrice:        "0.06",
				RootVolume:       model.NewGp2RootVolume(30),
			},
			{
				WeightedCapacity: 2,
				InstanceType:     "c4.xlarge",
				SpotPrice:        "0.12",
				RootVolume:       model.NewGp2RootVolume(60),
			},
		}
		p := c.NodePools[0]
		actual := p.NodePoolConfig.SpotFleet.LaunchSpecifications
		if !reflect.DeepEqual(expected, actual) {
			t.Errorf(
				"LaunchSpecifications didn't match: expected=%v actual=%v",
				expected,
				actual,
			)
		}
	}

	spotFleetBasedNodePoolHasWaitSignalDisabled := func(c *config.Config, t *testing.T) {
		expected := controlplane_config.WaitSignal{
			Enabled:      false,
			MaxBatchSize: 1,
		}
		p := c.NodePools[0]

		if !p.SpotFleet.Enabled() {
			t.Errorf("1st node pool is expected to be a spot fleet based one but was not: %+v", p)
		}

		actual := p.WaitSignal
		if !reflect.DeepEqual(expected, actual) {
			t.Errorf(
				"WaitSignal didn't match: expected=%v actual=%v",
				expected,
				actual,
			)
		}
	}

	asgBasedNodePoolHasWaitSignalEnabled := func(c *config.Config, t *testing.T) {
		expected := controlplane_config.WaitSignal{
			Enabled:      true,
			MaxBatchSize: 1,
		}
		p := c.NodePools[0]

		if p.SpotFleet.Enabled() {
			t.Errorf("1st node pool is expected to be an asg-based one but was not: %+v", p)
		}

		actual := p.WaitSignal
		if !reflect.DeepEqual(expected, actual) {
			t.Errorf(
				"WaitSignal didn't match: expected=%v actual=%v",
				expected,
				actual,
			)
		}
	}

	hasPrivateSubnetsWithManagedNGWs := func(numExpectedNum int) func(c *config.Config, t *testing.T) {
		return func(c *config.Config, t *testing.T) {
			for i, s := range c.PrivateSubnets() {
				if !s.ManageNATGateway() {
					t.Errorf("NAT gateway for the existing private subnet #%d should be created by kube-aws but was not", i)
				}

				if s.ManageRouteToInternet() {
					t.Errorf("Route to IGW shouldn't be created for a private subnet: %v", s)
				}
			}
		}
	}

	hasSpecificNumOfManagedNGWsWithUnmanagedEIPs := func(ngwExpectedNum int) func(c *config.Config, t *testing.T) {
		return func(c *config.Config, t *testing.T) {
			ngwActualNum := len(c.NATGateways())
			if ngwActualNum != ngwExpectedNum {
				t.Errorf("Number of NAT gateways(%d) doesn't match with the expexted one: %d", ngwActualNum, ngwExpectedNum)
			}
			for i, n := range c.NATGateways() {
				if !n.ManageNATGateway() {
					t.Errorf("NGW #%d is expected to be managed by kube-aws but was not: %+v", i, n)
				}
				if n.ManageEIP() {
					t.Errorf("EIP for NGW #%d is expected to be unmanaged by kube-aws but was not: %+v", i, n)
				}
				if !n.ManageRoute() {
					t.Errorf("Routes for NGW #%d is expected to be managed by kube-aws but was not: %+v", i, n)
				}
			}
		}
	}

	hasSpecificNumOfManagedNGWsAndEIPs := func(ngwExpectedNum int) func(c *config.Config, t *testing.T) {
		return func(c *config.Config, t *testing.T) {
			ngwActualNum := len(c.NATGateways())
			if ngwActualNum != ngwExpectedNum {
				t.Errorf("Number of NAT gateways(%d) doesn't match with the expexted one: %d", ngwActualNum, ngwExpectedNum)
			}
			for i, n := range c.NATGateways() {
				if !n.ManageNATGateway() {
					t.Errorf("NGW #%d is expected to be managed by kube-aws but was not: %+v", i, n)
				}
				if !n.ManageEIP() {
					t.Errorf("EIP for NGW #%d is expected to be managed by kube-aws but was not: %+v", i, n)
				}
				if !n.ManageRoute() {
					t.Errorf("Routes for NGW #%d is expected to be managed by kube-aws but was not: %+v", i, n)
				}
			}
		}
	}

	hasTwoManagedNGWsAndEIPs := hasSpecificNumOfManagedNGWsAndEIPs(2)

	hasNoManagedNGWsButSpecificNumOfRoutesToUnmanagedNGWs := func(ngwExpectedNum int) func(c *config.Config, t *testing.T) {
		return func(c *config.Config, t *testing.T) {
			ngwActualNum := len(c.NATGateways())
			if ngwActualNum != ngwExpectedNum {
				t.Errorf("Number of NAT gateways(%d) doesn't match with the expexted one: %d", ngwActualNum, ngwExpectedNum)
			}
			for i, n := range c.NATGateways() {
				if n.ManageNATGateway() {
					t.Errorf("NGW #%d is expected to be unamanged by kube-aws but was not: %+v", i, n)
				}
				if n.ManageEIP() {
					t.Errorf("EIP for NGW #%d is expected to be unmanaged by kube-aws but was not: %+v", i, n)
				}
				if !n.ManageRoute() {
					t.Errorf("Routes for NGW #%d is expected to be managed by kube-aws but was not: %+v", i, n)
				}
			}
		}
	}

	hasNoNGWsOrEIPsOrRoutes := func(c *config.Config, t *testing.T) {
		ngwActualNum := len(c.NATGateways())
		ngwExpectedNum := 0
		if ngwActualNum != ngwExpectedNum {
			t.Errorf("Number of NAT gateways(%d) doesn't match with the expexted one: %d", ngwActualNum, ngwExpectedNum)
		}
	}

	hasDefaultCluster := func(c root.Cluster, t *testing.T) {
		assets, err := c.Assets()
		if err != nil {
			t.Errorf("failed to list assets: %v", err)
			t.FailNow()
		}

		t.Run("Assets/RootStackTemplate", func(t *testing.T) {
			cluster := kubeAwsSettings.clusterName
			stack := kubeAwsSettings.clusterName
			file := "stack.json"
			expected := cfnstack.Asset{
				Content: "",
				AssetLocation: cfnstack.AssetLocation{
					ID:     cfnstack.NewAssetID(stack, file),
					Bucket: s3Bucket,
					Key:    s3Dir + "/kube-aws/clusters/" + cluster + "/exported/stacks/" + stack + "/" + file,
					Path:   stack + "/stack.json",
				},
			}
			actual, err := assets.FindAssetByStackAndFileName(stack, file)
			if err != nil {
				t.Errorf("failed to find asset: %v", err)
			}
			if expected.ID != actual.ID {
				t.Errorf(
					"Asset id didn't match: expected=%v actual=%v",
					expected.ID,
					actual.ID,
				)
			}
			if expected.Key != actual.Key {
				t.Errorf(
					"Asset key didn't match: expected=%v actual=%v",
					expected.Key,
					actual.Key,
				)
			}
		})

		t.Run("Assets/ControlplaneStackTemplate", func(t *testing.T) {
			cluster := kubeAwsSettings.clusterName
			stack := "control-plane"
			file := "stack.json"
			expected := cfnstack.Asset{
				Content: string(controlplane_config.StackTemplateTemplate),
				AssetLocation: cfnstack.AssetLocation{
					ID:     cfnstack.NewAssetID(stack, file),
					Bucket: s3Bucket,
					Key:    s3Dir + "/kube-aws/clusters/" + cluster + "/exported/stacks/" + stack + "/" + file,
					Path:   stack + "/stack.json",
				},
			}
			actual, err := assets.FindAssetByStackAndFileName(stack, file)
			if err != nil {
				t.Errorf("failed to find asset: %v", err)
			}
			if expected.ID != actual.ID {
				t.Errorf(
					"Asset id didn't match: expected=%v actual=%v",
					expected.ID,
					actual.ID,
				)
			}
			if expected.Key != actual.Key {
				t.Errorf(
					"Asset key didn't match: expected=%v actual=%v",
					expected.Key,
					actual.Key,
				)
			}
		})
	}

	minimalValidConfigYaml := kubeAwsSettings.mainClusterYaml + `
availabilityZone: us-west-1c
`
	validCases := []struct {
		context       string
		configYaml    string
		assertConfig  []ConfigTester
		assertCluster []ClusterTester
	}{
		{
			context: "WithCustomSettings",
			configYaml: minimalValidConfigYaml + `
customSettings:
  stack-type: control-plane
worker:
  nodePools:
  - name: pool1
    customSettings:
      stack-type: node-pool
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				asgBasedNodePoolHasWaitSignalEnabled,
				func(c *config.Config, t *testing.T) {
					p := c.NodePools[0]

					{
						expected := map[string]interface{}{
							"stack-type": "control-plane",
						}
						actual := c.CustomSettings
						if !reflect.DeepEqual(expected, actual) {
							t.Errorf("customSettings didn't match : expected=%v actual=%v", expected, actual)
						}
					}

					{
						expected := map[string]interface{}{
							"stack-type": "node-pool",
						}
						actual := p.CustomSettings
						if !reflect.DeepEqual(expected, actual) {
							t.Errorf("customSettings didn't match : expected=%v actual=%v", expected, actual)
						}
					}
				},
			},
			assertCluster: []ClusterTester{
				hasDefaultCluster,
			},
		},
		{
			context: "WithEtcdMemberIdentityProviderEIP",
			configYaml: minimalValidConfigYaml + `
etcd:
  memberIdentityProvider: eip
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					subnet1 := model.NewPublicSubnet("us-west-1c", "10.0.0.0/24")
					subnet1.Name = "Subnet0"
					expected := controlplane_config.EtcdSettings{
						Etcd: model.Etcd{
							Cluster: model.EtcdCluster{
								MemberIdentityProvider: "eip",
							},
							Subnets: []model.Subnet{
								subnet1,
							},
						},
						EtcdCount:               1,
						EtcdInstanceType:        "t2.medium",
						EtcdRootVolumeSize:      30,
						EtcdRootVolumeType:      "gp2",
						EtcdRootVolumeIOPS:      0,
						EtcdDataVolumeSize:      30,
						EtcdDataVolumeType:      "gp2",
						EtcdDataVolumeIOPS:      0,
						EtcdDataVolumeEphemeral: false,
						EtcdTenancy:             "default",
					}
					actual := c.EtcdSettings
					if !reflect.DeepEqual(expected, actual) {
						t.Errorf(
							"EtcdSettings didn't match: expected=%v actual=%v",
							expected,
							actual,
						)
					}

					if !actual.NodeShouldHaveEIP() {
						t.Errorf(
							"NodeShouldHaveEIP returned unexpected value: %v",
							actual.NodeShouldHaveEIP(),
						)
					}
				},
			},
			assertCluster: []ClusterTester{
				hasDefaultCluster,
			},
		},
		{
			context: "WithEtcdMemberIdentityProviderENI",
			configYaml: minimalValidConfigYaml + `
etcd:
  memberIdentityProvider: eni
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					subnet1 := model.NewPublicSubnet("us-west-1c", "10.0.0.0/24")
					subnet1.Name = "Subnet0"
					expected := controlplane_config.EtcdSettings{
						Etcd: model.Etcd{
							Cluster: model.EtcdCluster{
								MemberIdentityProvider: "eni",
							},
							Subnets: []model.Subnet{
								subnet1,
							},
						},
						EtcdCount:               1,
						EtcdInstanceType:        "t2.medium",
						EtcdRootVolumeSize:      30,
						EtcdRootVolumeType:      "gp2",
						EtcdRootVolumeIOPS:      0,
						EtcdDataVolumeSize:      30,
						EtcdDataVolumeType:      "gp2",
						EtcdDataVolumeIOPS:      0,
						EtcdDataVolumeEphemeral: false,
						EtcdTenancy:             "default",
					}
					actual := c.EtcdSettings
					if !reflect.DeepEqual(expected, actual) {
						t.Errorf(
							"EtcdSettings didn't match: expected=%v actual=%v",
							expected,
							actual,
						)
					}

					if !actual.NodeShouldHaveSecondaryENI() {
						t.Errorf(
							"NodeShouldHaveSecondaryENI returned unexpected value: %v",
							actual.NodeShouldHaveSecondaryENI(),
						)
					}
				},
			},
			assertCluster: []ClusterTester{
				hasDefaultCluster,
			},
		},
		{
			context: "WithEtcdMemberIdentityProviderENIWithCustomDomain",
			configYaml: minimalValidConfigYaml + `
etcd:
  memberIdentityProvider: eni
  internalDomainName: internal.example.com
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					subnet1 := model.NewPublicSubnet("us-west-1c", "10.0.0.0/24")
					subnet1.Name = "Subnet0"
					expected := controlplane_config.EtcdSettings{
						Etcd: model.Etcd{
							Cluster: model.EtcdCluster{
								MemberIdentityProvider: "eni",
								InternalDomainName:     "internal.example.com",
							},
							Subnets: []model.Subnet{
								subnet1,
							},
						},
						EtcdCount:               1,
						EtcdInstanceType:        "t2.medium",
						EtcdRootVolumeSize:      30,
						EtcdRootVolumeType:      "gp2",
						EtcdRootVolumeIOPS:      0,
						EtcdDataVolumeSize:      30,
						EtcdDataVolumeType:      "gp2",
						EtcdDataVolumeIOPS:      0,
						EtcdDataVolumeEphemeral: false,
						EtcdTenancy:             "default",
					}
					actual := c.EtcdSettings
					if !reflect.DeepEqual(expected, actual) {
						t.Errorf(
							"EtcdSettings didn't match: expected=%v actual=%v",
							expected,
							actual,
						)
					}

					if !actual.NodeShouldHaveSecondaryENI() {
						t.Errorf(
							"NodeShouldHaveSecondaryENI returned unexpected value: %v",
							actual.NodeShouldHaveSecondaryENI(),
						)
					}
				},
			},
			assertCluster: []ClusterTester{
				hasDefaultCluster,
			},
		},
		{
			context: "WithEtcdMemberIdentityProviderENIWithCustomFQDNs",
			configYaml: minimalValidConfigYaml + `
etcd:
  memberIdentityProvider: eni
  internalDomainName: internal.example.com
  nodes:
  - fqdn: etcd1a.internal.example.com
  - fqdn: etcd1b.internal.example.com
  - fqdn: etcd1c.internal.example.com
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					subnet1 := model.NewPublicSubnet("us-west-1c", "10.0.0.0/24")
					subnet1.Name = "Subnet0"
					expected := controlplane_config.EtcdSettings{
						Etcd: model.Etcd{
							Cluster: model.EtcdCluster{
								MemberIdentityProvider: "eni",
								InternalDomainName:     "internal.example.com",
							},
							Nodes: []model.EtcdNode{
								model.EtcdNode{
									FQDN: "etcd1a.internal.example.com",
								},
								model.EtcdNode{
									FQDN: "etcd1b.internal.example.com",
								},
								model.EtcdNode{
									FQDN: "etcd1c.internal.example.com",
								},
							},
							Subnets: []model.Subnet{
								subnet1,
							},
						},
						EtcdCount:               1,
						EtcdInstanceType:        "t2.medium",
						EtcdRootVolumeSize:      30,
						EtcdRootVolumeType:      "gp2",
						EtcdRootVolumeIOPS:      0,
						EtcdDataVolumeSize:      30,
						EtcdDataVolumeType:      "gp2",
						EtcdDataVolumeIOPS:      0,
						EtcdDataVolumeEphemeral: false,
						EtcdTenancy:             "default",
					}
					actual := c.EtcdSettings
					if !reflect.DeepEqual(expected, actual) {
						t.Errorf(
							"EtcdSettings didn't match: expected=%v actual=%v",
							expected,
							actual,
						)
					}

					if !actual.NodeShouldHaveSecondaryENI() {
						t.Errorf(
							"NodeShouldHaveSecondaryENI returned unexpected value: %v",
							actual.NodeShouldHaveSecondaryENI(),
						)
					}
				},
			},
			assertCluster: []ClusterTester{
				hasDefaultCluster,
			},
		},
		{
			context: "WithEtcdMemberIdentityProviderENIWithCustomNames",
			configYaml: minimalValidConfigYaml + `
etcd:
  memberIdentityProvider: eni
  internalDomainName: internal.example.com
  nodes:
  - name: etcd1a
  - name: etcd1b
  - name: etcd1c
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					subnet1 := model.NewPublicSubnet("us-west-1c", "10.0.0.0/24")
					subnet1.Name = "Subnet0"
					expected := controlplane_config.EtcdSettings{
						Etcd: model.Etcd{
							Cluster: model.EtcdCluster{
								MemberIdentityProvider: "eni",
								InternalDomainName:     "internal.example.com",
							},
							Nodes: []model.EtcdNode{
								model.EtcdNode{
									Name: "etcd1a",
								},
								model.EtcdNode{
									Name: "etcd1b",
								},
								model.EtcdNode{
									Name: "etcd1c",
								},
							},
							Subnets: []model.Subnet{
								subnet1,
							},
						},
						EtcdCount:               1,
						EtcdInstanceType:        "t2.medium",
						EtcdRootVolumeSize:      30,
						EtcdRootVolumeType:      "gp2",
						EtcdRootVolumeIOPS:      0,
						EtcdDataVolumeSize:      30,
						EtcdDataVolumeType:      "gp2",
						EtcdDataVolumeIOPS:      0,
						EtcdDataVolumeEphemeral: false,
						EtcdTenancy:             "default",
					}
					actual := c.EtcdSettings
					if !reflect.DeepEqual(expected, actual) {
						t.Errorf(
							"EtcdSettings didn't match: expected=%v actual=%v",
							expected,
							actual,
						)
					}

					if !actual.NodeShouldHaveSecondaryENI() {
						t.Errorf(
							"NodeShouldHaveSecondaryENI returned unexpected value: %v",
							actual.NodeShouldHaveSecondaryENI(),
						)
					}
				},
			},
			assertCluster: []ClusterTester{
				hasDefaultCluster,
			},
		},
		{
			context: "WithEtcdMemberIdentityProviderENIWithoutRecordSets",
			configYaml: minimalValidConfigYaml + `
etcd:
  memberIdentityProvider: eni
  internalDomainName: internal.example.com
  manageRecordSets: false
  nodes:
  - name: etcd1a
  - name: etcd1b
  - name: etcd1c
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					subnet1 := model.NewPublicSubnet("us-west-1c", "10.0.0.0/24")
					subnet1.Name = "Subnet0"
					manageRecordSets := false
					expected := controlplane_config.EtcdSettings{
						Etcd: model.Etcd{
							Cluster: model.EtcdCluster{
								ManageRecordSets:       &manageRecordSets,
								MemberIdentityProvider: "eni",
								InternalDomainName:     "internal.example.com",
							},
							Nodes: []model.EtcdNode{
								model.EtcdNode{
									Name: "etcd1a",
								},
								model.EtcdNode{
									Name: "etcd1b",
								},
								model.EtcdNode{
									Name: "etcd1c",
								},
							},
							Subnets: []model.Subnet{
								subnet1,
							},
						},
						EtcdCount:               1,
						EtcdInstanceType:        "t2.medium",
						EtcdRootVolumeSize:      30,
						EtcdRootVolumeType:      "gp2",
						EtcdRootVolumeIOPS:      0,
						EtcdDataVolumeSize:      30,
						EtcdDataVolumeType:      "gp2",
						EtcdDataVolumeIOPS:      0,
						EtcdDataVolumeEphemeral: false,
						EtcdTenancy:             "default",
					}
					actual := c.EtcdSettings
					if !reflect.DeepEqual(expected, actual) {
						t.Errorf(
							"EtcdSettings didn't match: expected=%v actual=%v",
							expected,
							actual,
						)
					}

					if !actual.NodeShouldHaveSecondaryENI() {
						t.Errorf(
							"NodeShouldHaveSecondaryENI returned unexpected value: %v",
							actual.NodeShouldHaveSecondaryENI(),
						)
					}
				},
			},
			assertCluster: []ClusterTester{
				hasDefaultCluster,
			},
		},
		{
			context: "WithEtcdMemberIdentityProviderENIWithHostedZoneID",
			configYaml: minimalValidConfigYaml + `
etcd:
  memberIdentityProvider: eni
  internalDomainName: internal.example.com
  hostedZone:
    id: hostedzone-abcdefg
  nodes:
  - name: etcd1a
  - name: etcd1b
  - name: etcd1c
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					subnet1 := model.NewPublicSubnet("us-west-1c", "10.0.0.0/24")
					subnet1.Name = "Subnet0"
					expected := controlplane_config.EtcdSettings{
						Etcd: model.Etcd{
							Cluster: model.EtcdCluster{
								HostedZone:             model.Identifier{ID: "hostedzone-abcdefg"},
								MemberIdentityProvider: "eni",
								InternalDomainName:     "internal.example.com",
							},
							Nodes: []model.EtcdNode{
								model.EtcdNode{
									Name: "etcd1a",
								},
								model.EtcdNode{
									Name: "etcd1b",
								},
								model.EtcdNode{
									Name: "etcd1c",
								},
							},
							Subnets: []model.Subnet{
								subnet1,
							},
						},
						EtcdCount:               1,
						EtcdInstanceType:        "t2.medium",
						EtcdRootVolumeSize:      30,
						EtcdRootVolumeType:      "gp2",
						EtcdRootVolumeIOPS:      0,
						EtcdDataVolumeSize:      30,
						EtcdDataVolumeType:      "gp2",
						EtcdDataVolumeIOPS:      0,
						EtcdDataVolumeEphemeral: false,
						EtcdTenancy:             "default",
					}
					actual := c.EtcdSettings
					if !reflect.DeepEqual(expected, actual) {
						t.Errorf(
							"EtcdSettings didn't match: expected=%v actual=%v",
							expected,
							actual,
						)
					}

					if !actual.NodeShouldHaveSecondaryENI() {
						t.Errorf(
							"NodeShouldHaveSecondaryENI returned unexpected value: %v",
							actual.NodeShouldHaveSecondaryENI(),
						)
					}
				},
			},
			assertCluster: []ClusterTester{
				hasDefaultCluster,
			},
		},
		{
			context: "WithExperimentalFeatures",
			configYaml: minimalValidConfigYaml + `
experimental:
  admission:
    podSecurityPolicy:
      enabled: true
  auditLog:
    enabled: true
    maxage: 100
    logpath: "/var/log/audit.log"
  authentication:
    webhook:
      enabled: true
      cacheTTL: "1234s"
      configBase64: "e30k"
  awsEnvironment:
    enabled: true
    environment:
      CFNSTACK: '{ "Ref" : "AWS::StackId" }'
  awsNodeLabels:
    enabled: true
  clusterAutoscalerSupport:
    enabled: true
  ephemeralImageStorage:
    enabled: true
  kube2IamSupport:
    enabled: true
  loadBalancer:
    enabled: true
    names:
      - manuallymanagedlb
    securityGroupIds:
      - sg-12345678
  targetGroup:
    enabled: true
    arns:
      - arn:aws:elasticloadbalancing:eu-west-1:xxxxxxxxxxxx:targetgroup/manuallymanagedetg/xxxxxxxxxxxxxxxx
    securityGroupIds:
      - sg-12345678
  nodeDrainer:
    enabled: true
  nodeLabels:
    kube-aws.coreos.com/role: worker
  plugins:
    rbac:
      enabled: true
  taints:
    - key: reservation
      value: spot
      effect: NoSchedule
worker:
  nodePools:
  - name: pool1
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				asgBasedNodePoolHasWaitSignalEnabled,
				func(c *config.Config, t *testing.T) {
					expected := controlplane_config.Experimental{
						Admission: controlplane_config.Admission{
							PodSecurityPolicy: controlplane_config.PodSecurityPolicy{
								Enabled: true,
							},
						},
						AuditLog: controlplane_config.AuditLog{
							Enabled: true,
							MaxAge:  100,
							LogPath: "/var/log/audit.log",
						},
						Authentication: controlplane_config.Authentication{
							Webhook: controlplane_config.Webhook{
								Enabled:  true,
								CacheTTL: "1234s",
								Config:   "e30k",
							},
						},
						AwsEnvironment: controlplane_config.AwsEnvironment{
							Enabled: true,
							Environment: map[string]string{
								"CFNSTACK": `{ "Ref" : "AWS::StackId" }`,
							},
						},
						AwsNodeLabels: controlplane_config.AwsNodeLabels{
							Enabled: true,
						},
						ClusterAutoscalerSupport: controlplane_config.ClusterAutoscalerSupport{
							Enabled: true,
						},
						EphemeralImageStorage: controlplane_config.EphemeralImageStorage{
							Enabled:    true,
							Disk:       "xvdb",
							Filesystem: "xfs",
						},
						Kube2IamSupport: controlplane_config.Kube2IamSupport{
							Enabled: true,
						},
						LoadBalancer: controlplane_config.LoadBalancer{
							Enabled:          true,
							Names:            []string{"manuallymanagedlb"},
							SecurityGroupIds: []string{"sg-12345678"},
						},
						TargetGroup: controlplane_config.TargetGroup{
							Enabled:          true,
							Arns:             []string{"arn:aws:elasticloadbalancing:eu-west-1:xxxxxxxxxxxx:targetgroup/manuallymanagedetg/xxxxxxxxxxxxxxxx"},
							SecurityGroupIds: []string{"sg-12345678"},
						},
						NodeDrainer: controlplane_config.NodeDrainer{
							Enabled: true,
						},
						NodeLabels: controlplane_config.NodeLabels{
							"kube-aws.coreos.com/role": "worker",
						},
						Plugins: controlplane_config.Plugins{
							Rbac: controlplane_config.Rbac{
								Enabled: true,
							},
						},
						Taints: []controlplane_config.Taint{
							{Key: "reservation", Value: "spot", Effect: "NoSchedule"},
						},
					}

					actual := c.Experimental

					if !reflect.DeepEqual(expected, actual) {
						t.Errorf("experimental settings didn't match : expected=%v actual=%v", expected, actual)
					}

					p := c.NodePools[0]
					if reflect.DeepEqual(expected, p.Experimental) {
						t.Errorf("experimental settings shouldn't be inherited to a node pool but it did : toplevel=%v nodepool=%v", expected, p.Experimental)
					}
				},
			},
			assertCluster: []ClusterTester{
				hasDefaultCluster,
			},
		},
		{
			context: "WithExperimentalFeaturesForWorkerNodePool",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    admission:
      podSecurityPolicy:
        enabled: true
    auditLog:
      enabled: true
      maxage: 100
      logpath: "/var/log/audit.log"
    awsEnvironment:
      enabled: true
      environment:
        CFNSTACK: '{ "Ref" : "AWS::StackId" }'
    awsNodeLabels:
      enabled: true
    clusterAutoscalerSupport:
      enabled: true
    ephemeralImageStorage:
      enabled: true
    kube2IamSupport:
      enabled: true
    loadBalancer:
      enabled: true
      names:
        - manuallymanagedlb
      securityGroupIds:
        - sg-12345678
    targetGroup:
      enabled: true
      arns:
        - arn:aws:elasticloadbalancing:eu-west-1:xxxxxxxxxxxx:targetgroup/manuallymanagedetg/xxxxxxxxxxxxxxxx
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
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				asgBasedNodePoolHasWaitSignalEnabled,
				func(c *config.Config, t *testing.T) {
					expected := controlplane_config.Experimental{
						AwsEnvironment: controlplane_config.AwsEnvironment{
							Enabled: true,
							Environment: map[string]string{
								"CFNSTACK": `{ "Ref" : "AWS::StackId" }`,
							},
						},
						AwsNodeLabels: controlplane_config.AwsNodeLabels{
							Enabled: true,
						},
						ClusterAutoscalerSupport: controlplane_config.ClusterAutoscalerSupport{
							Enabled: true,
						},
						EphemeralImageStorage: controlplane_config.EphemeralImageStorage{
							Enabled:    true,
							Disk:       "xvdb",
							Filesystem: "xfs",
						},
						Kube2IamSupport: controlplane_config.Kube2IamSupport{
							Enabled: true,
						},
						LoadBalancer: controlplane_config.LoadBalancer{
							Enabled:          true,
							Names:            []string{"manuallymanagedlb"},
							SecurityGroupIds: []string{"sg-12345678"},
						},
						TargetGroup: controlplane_config.TargetGroup{
							Enabled:          true,
							Arns:             []string{"arn:aws:elasticloadbalancing:eu-west-1:xxxxxxxxxxxx:targetgroup/manuallymanagedetg/xxxxxxxxxxxxxxxx"},
							SecurityGroupIds: []string{"sg-12345678"},
						},
						NodeDrainer: controlplane_config.NodeDrainer{
							Enabled: true,
						},
						NodeLabels: controlplane_config.NodeLabels{
							"kube-aws.coreos.com/role": "worker",
						},
						Taints: []controlplane_config.Taint{
							{Key: "reservation", Value: "spot", Effect: "NoSchedule"},
						},
					}
					p := c.NodePools[0]
					if reflect.DeepEqual(expected, p.Experimental) {
						t.Errorf("experimental settings for node pool didn't match : expected=%v actual=%v", expected, p.Experimental)
					}
				},
			},
		},
		{
			context: "WithKube2IamSupport",
			configYaml: minimalValidConfigYaml + `
controller:
  managedIamRoleName: mycontrollerrole
experimental:
  kube2IamSupport:
    enabled: true
worker:
  nodePools:
  - name: pool1
    managedIamRoleName: myworkerrole
    kube2IamSupport:
      enabled: true
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				asgBasedNodePoolHasWaitSignalEnabled,
				func(c *config.Config, t *testing.T) {
					expectedControllerRoleName := "mycontrollerrole"
					expectedWorkerRoleName := "myworkerrole"

					if expectedControllerRoleName != c.Controller.ManagedIamRoleName {
						t.Errorf("controller's managedIamRoleName didn't match : expected=%v actual=%v", expectedControllerRoleName, c.Controller.ManagedIamRoleName)
					}

					if !c.Experimental.Kube2IamSupport.Enabled {
						t.Errorf("controller's experimental.kube2IamSupport should be enabled but was not: %+v", c.Experimental)
					}

					p := c.NodePools[0]
					if expectedWorkerRoleName != p.ManagedIamRoleName {
						t.Errorf("worker node pool's managedIamRoleName didn't match : expected=%v actual=%v", expectedWorkerRoleName, p.ManagedIamRoleName)
					}

					if !p.Kube2IamSupport.Enabled {
						t.Errorf("worker node pool's kube2IamSupport should be enabled but was not: %+v", p.Experimental)
					}
				},
			},
		},
		{
			context: "WithNodePoolWithWaitSignalEnabled",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    waitSignal:
      enabled: true
  - name: pool2
    waitSignal:
      enabled: true
      maxBatchSize: 2
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				func(c *config.Config, t *testing.T) {
					if !c.NodePools[0].WaitSignal.Enabled {
						t.Errorf("waitSignal should be enabled for node pool at index %d but was not", 0)
					}
					if c.NodePools[0].WaitSignal.MaxBatchSize != 1 {
						t.Errorf("waitSignal.maxBatchSize should be 1 for node pool at index %d but was %d", 0, c.NodePools[0].WaitSignal.MaxBatchSize)
					}
					if !c.NodePools[1].WaitSignal.Enabled {
						t.Errorf("waitSignal should be enabled for node pool at index %d but was not", 1)
					}
					if c.NodePools[1].WaitSignal.MaxBatchSize != 2 {
						t.Errorf("waitSignal.maxBatchSize should be 2 for node pool at index %d but was %d", 1, c.NodePools[1].WaitSignal.MaxBatchSize)
					}
				},
			},
		},
		{
			context:    "WithMinimalValidConfig",
			configYaml: minimalValidConfigYaml,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				hasDefaultExperimentalFeatures,
			},
		},
		{
			context: "WithVaryingWorkerCountPerNodePool",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
  - name: pool2
    count: 2
  - name: pool3
    count: 0
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				hasDefaultExperimentalFeatures,
				func(c *config.Config, t *testing.T) {
					if *c.NodePools[0].Count != 1 {
						t.Errorf("default worker count should be 1 but was: %d", c.NodePools[0].Count)
					}
					if *c.NodePools[1].Count != 2 {
						t.Errorf("worker count should be set to 2 but was: %d", c.NodePools[1].Count)
					}
					if *c.NodePools[2].Count != 0 {
						t.Errorf("worker count should be be set to 0 but was: %d", c.NodePools[2].Count)
					}
				},
			},
		},
		{
			context: "WithVaryingWorkerASGSizePerNodePool",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
  - name: pool2
    count: 2
  - name: pool3
    autoScalingGroup:
      minSize: 0
      maxSize: 10
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				hasDefaultExperimentalFeatures,
				func(c *config.Config, t *testing.T) {
					if c.NodePools[0].MaxCount() != 1 {
						t.Errorf("worker max count should be 1 but was: %d", c.NodePools[0].MaxCount())
					}
					if c.NodePools[0].MinCount() != 1 {
						t.Errorf("worker min count should be 1 but was: %d", c.NodePools[0].MinCount())
					}
					if c.NodePools[1].MaxCount() != 2 {
						t.Errorf("worker max count should be 2 but was: %d", c.NodePools[1].MaxCount())
					}
					if c.NodePools[1].MinCount() != 2 {
						t.Errorf("worker min count should be 2 but was: %d", c.NodePools[1].MinCount())
					}
					if c.NodePools[2].MaxCount() != 10 {
						t.Errorf("worker max count should be 10 but was: %d", c.NodePools[2].MaxCount())
					}
					if c.NodePools[2].MinCount() != 0 {
						t.Errorf("worker min count should be 0 but was: %d", c.NodePools[2].MinCount())
					}
				},
			},
		},
		{
			context: "WithNetworkTopologyAllPreconfiguredPrivateDeprecated",
			configYaml: kubeAwsSettings.mainClusterYaml + `
vpcId: vpc-1a2b3c4d
# This, in combination with mapPublicIPs=false, implies that the route table contains a route to a preconfigured NAT gateway
# See https://github.com/coreos/kube-aws/pull/284#issuecomment-276008202
routeTableId: rtb-1a2b3c4d
# This means that all the subnets created by kube-aws should be private
mapPublicIPs: false
# This can't be false because kube-aws won't create public subbnets which are required by an external lb when mapPublicIPs=false
controllerLoadBalancerPrivate: true
subnets:
- availabilityZone: us-west-1a
  instanceCIDR: "10.0.1.0/24"
  # implies
  # private: true
  # routeTable
  #   id: rtb-1a2b3c4d
- availabilityZone: us-west-1b
  instanceCIDR: "10.0.2.0/24"
  # implies
  # private: true
  # routeTable
  #   id: rtb-1a2b3c4d
`,
			assertConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				hasNoNGWsOrEIPsOrRoutes,
				func(c *config.Config, t *testing.T) {
					private1 := model.NewPrivateSubnetWithPreconfiguredRouteTable("us-west-1a", "10.0.1.0/24", "rtb-1a2b3c4d")
					private1.Name = "Subnet0"

					private2 := model.NewPrivateSubnetWithPreconfiguredRouteTable("us-west-1b", "10.0.2.0/24", "rtb-1a2b3c4d")
					private2.Name = "Subnet1"

					subnets := []model.Subnet{
						private1,
						private2,
					}
					if !reflect.DeepEqual(c.AllSubnets(), subnets) {
						t.Errorf("Managed subnets didn't match: expected=%+v actual=%+v", subnets, c.AllSubnets())
					}

					privateSubnets := []model.Subnet{
						private1,
						private2,
					}
					if !reflect.DeepEqual(c.Controller.Subnets, privateSubnets) {
						t.Errorf("Controller subnets didn't match: expected=%+v actual=%+v", privateSubnets, c.Controller.Subnets)
					}
					if !reflect.DeepEqual(c.Controller.LoadBalancer.Subnets, privateSubnets) {
						t.Errorf("Controller loadbalancer subnets didn't match: expected=%+v actual=%+v", privateSubnets, c.Controller.LoadBalancer.Subnets)
					}
					if !reflect.DeepEqual(c.Etcd.Subnets, privateSubnets) {
						t.Errorf("Etcd subnets didn't match: expected=%+v actual=%+v", privateSubnets, c.Etcd.Subnets)
					}

					for i, s := range c.PrivateSubnets() {
						if s.ManageNATGateway() {
							t.Errorf("NAT gateway for the private subnet #%d is externally managed and shouldn't created by kube-aws", i)
						}

						if s.ManageRouteToInternet() {
							t.Errorf("Route to IGW shouldn't be created for a private subnet: %+v", s)
						}
					}

					if len(c.PublicSubnets()) != 0 {
						t.Errorf("Number of public subnets should be zero but it wasn't: %d", len(c.PublicSubnets()))
					}
				},
			},
		},
		{
			context: "WithNetworkTopologyAllPreconfiguredPublicDeprecated",
			configYaml: kubeAwsSettings.mainClusterYaml + `
vpcId: vpc-1a2b3c4d
# This, in combination with mapPublicIPs=true, implies that the route table contains a route to a preconfigured internet gateway
# See https://github.com/coreos/kube-aws/pull/284#issuecomment-276008202
routeTableId: rtb-1a2b3c4d
# This means that all the subnets created by kube-aws should be public
mapPublicIPs: true
# This can't be true because kube-aws won't create private subnets which are required by an internal lb when mapPublicIPs=true
controllerLoadBalancerPrivate: false
# internetGatewayId should be omitted as we assume that the route table specified by routeTableId already contain a route to one
#internetGatewayId:
subnets:
- availabilityZone: us-west-1a
  instanceCIDR: "10.0.1.0/24"
  # #implies
  # private: false
  # routeTable
  #   id: rtb-1a2b3c4d
- availabilityZone: us-west-1b
  instanceCIDR: "10.0.2.0/24"
  # #implies
  # private: false
  # routeTable
  #   id: rtb-1a2b3c4d
`,
			assertConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				hasNoNGWsOrEIPsOrRoutes,
				func(c *config.Config, t *testing.T) {
					private1 := model.NewPublicSubnetWithPreconfiguredRouteTable("us-west-1a", "10.0.1.0/24", "rtb-1a2b3c4d")
					private1.Name = "Subnet0"

					private2 := model.NewPublicSubnetWithPreconfiguredRouteTable("us-west-1b", "10.0.2.0/24", "rtb-1a2b3c4d")
					private2.Name = "Subnet1"

					subnets := []model.Subnet{
						private1,
						private2,
					}
					if !reflect.DeepEqual(c.AllSubnets(), subnets) {
						t.Errorf("Managed subnets didn't match: expected=%+v actual=%+v", subnets, c.AllSubnets())
					}

					publicSubnets := []model.Subnet{
						private1,
						private2,
					}
					if !reflect.DeepEqual(c.Controller.Subnets, publicSubnets) {
						t.Errorf("Controller subnets didn't match: expected=%+v actual=%+v", publicSubnets, c.Controller.Subnets)
					}
					if !reflect.DeepEqual(c.Controller.LoadBalancer.Subnets, publicSubnets) {
						t.Errorf("Controller loadbalancer subnets didn't match: expected=%+v actual=%+v", publicSubnets, c.Controller.LoadBalancer.Subnets)
					}
					if !reflect.DeepEqual(c.Etcd.Subnets, publicSubnets) {
						t.Errorf("Etcd subnets didn't match: expected=%+v actual=%+v", publicSubnets, c.Etcd.Subnets)
					}

					for i, s := range c.PublicSubnets() {
						if s.RouteTableID() != "rtb-1a2b3c4d" {
							t.Errorf("Subnet %d should be associated to a route table with an IGW preconfigured but it wasn't", i)
						}

						if s.ManageRouteToInternet() {
							t.Errorf("Route to IGW shouldn't be created for a public subnet with a preconfigured IGW: %+v", s)
						}
					}

					if len(c.PrivateSubnets()) != 0 {
						t.Errorf("Number of private subnets should be zero but it wasn't: %d", len(c.PrivateSubnets()))
					}
				},
			},
		},
		{
			context: "WithNetworkTopologyExplicitSubnets",
			configYaml: kubeAwsSettings.mainClusterYaml + `
vpcId: vpc-1a2b3c4d
# routeTableId must be omitted
# See https://github.com/coreos/kube-aws/pull/284#issuecomment-275962332
# routeTableId: rtb-1a2b3c4d
subnets:
- name: private1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.1.0/24"
  private: true
- name: private2
  availabilityZone: us-west-1b
  instanceCIDR: "10.0.2.0/24"
  private: true
- name: public1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.3.0/24"
- name: public2
  availabilityZone: us-west-1b
  instanceCIDR: "10.0.4.0/24"
controller:
  subnets:
  - name: private1
  - name: private2
  loadBalancer:
    subnets:
    - name: public1
    - name: public2
    private: false
etcd:
  subnets:
  - name: private1
  - name: private2
worker:
  nodePools:
  - name: pool1
    subnets:
    - name: public1
    - name: public2
`,
			assertConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				everyPublicSubnetHasRouteToIGW,
				hasTwoManagedNGWsAndEIPs,
				func(c *config.Config, t *testing.T) {
					private1 := model.NewPrivateSubnet("us-west-1a", "10.0.1.0/24")
					private1.Name = "private1"

					private2 := model.NewPrivateSubnet("us-west-1b", "10.0.2.0/24")
					private2.Name = "private2"

					public1 := model.NewPublicSubnet("us-west-1a", "10.0.3.0/24")
					public1.Name = "public1"

					public2 := model.NewPublicSubnet("us-west-1b", "10.0.4.0/24")
					public2.Name = "public2"

					subnets := []model.Subnet{
						private1,
						private2,
						public1,
						public2,
					}
					if !reflect.DeepEqual(c.AllSubnets(), subnets) {
						t.Errorf("Managed subnets didn't match: expected=%v actual=%v", subnets, c.AllSubnets())
					}

					publicSubnets := []model.Subnet{
						public1,
						public2,
					}
					importedPublicSubnets := []model.Subnet{
						model.NewPublicSubnetFromFn("us-west-1a", `{"Fn::ImportValue":{"Fn::Sub":"${ControlPlaneStackName}-Public1"}}`),
						model.NewPublicSubnetFromFn("us-west-1b", `{"Fn::ImportValue":{"Fn::Sub":"${ControlPlaneStackName}-Public2"}}`),
					}

					p := c.NodePools[0]
					if !reflect.DeepEqual(p.Subnets, importedPublicSubnets) {
						t.Errorf("Worker subnets didn't match: expected=%v actual=%v", importedPublicSubnets, p.Subnets)
					}

					privateSubnets := []model.Subnet{
						private1,
						private2,
					}
					if !reflect.DeepEqual(c.Controller.Subnets, privateSubnets) {
						t.Errorf("Controller subnets didn't match: expected=%v actual=%v", privateSubnets, c.Controller.Subnets)
					}
					if !reflect.DeepEqual(c.Controller.LoadBalancer.Subnets, publicSubnets) {
						t.Errorf("Controller loadbalancer subnets didn't match: expected=%v actual=%v", publicSubnets, c.Controller.LoadBalancer.Subnets)
					}
					if !reflect.DeepEqual(c.Etcd.Subnets, privateSubnets) {
						t.Errorf("Etcd subnets didn't match: expected=%v actual=%v", privateSubnets, c.Etcd.Subnets)
					}

					for i, s := range c.PrivateSubnets() {
						if !s.ManageNATGateway() {
							t.Errorf("NAT gateway for the private subnet #%d should be created by kube-aws but it is not going to be", i)
						}

						if s.ManageRouteToInternet() {
							t.Errorf("Route to IGW shouldn't be created for a private subnet: %v", s)
						}
					}
				},
			},
		},
		{
			context: "WithNetworkTopologyImplicitSubnets",
			configYaml: kubeAwsSettings.mainClusterYaml + `
vpcId: vpc-1a2b3c4d
# routeTableId must be omitted
# See https://github.com/coreos/kube-aws/pull/284#issuecomment-275962332
# routeTableId: rtb-1a2b3c4d
subnets:
- name: private1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.1.0/24"
  private: true
- name: private2
  availabilityZone: us-west-1b
  instanceCIDR: "10.0.2.0/24"
  private: true
- name: public1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.3.0/24"
- name: public2
  availabilityZone: us-west-1b
  instanceCIDR: "10.0.4.0/24"
`,
			assertConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				everyPublicSubnetHasRouteToIGW,
				hasTwoManagedNGWsAndEIPs,
				func(c *config.Config, t *testing.T) {
					private1 := model.NewPrivateSubnet("us-west-1a", "10.0.1.0/24")
					private1.Name = "private1"

					private2 := model.NewPrivateSubnet("us-west-1b", "10.0.2.0/24")
					private2.Name = "private2"

					public1 := model.NewPublicSubnet("us-west-1a", "10.0.3.0/24")
					public1.Name = "public1"

					public2 := model.NewPublicSubnet("us-west-1b", "10.0.4.0/24")
					public2.Name = "public2"

					subnets := []model.Subnet{
						private1,
						private2,
						public1,
						public2,
					}
					if !reflect.DeepEqual(c.AllSubnets(), subnets) {
						t.Errorf("Managed subnets didn't match: expected=%v actual=%v", subnets, c.AllSubnets())
					}

					publicSubnets := []model.Subnet{
						public1,
						public2,
					}

					if !reflect.DeepEqual(c.Controller.Subnets, publicSubnets) {
						t.Errorf("Controller subnets didn't match: expected=%v actual=%v", publicSubnets, c.Controller.Subnets)
					}
					if !reflect.DeepEqual(c.Controller.LoadBalancer.Subnets, publicSubnets) {
						t.Errorf("Controller loadbalancer subnets didn't match: expected=%v actual=%v", publicSubnets, c.Controller.LoadBalancer.Subnets)
					}
					if !reflect.DeepEqual(c.Etcd.Subnets, publicSubnets) {
						t.Errorf("Etcd subnets didn't match: expected=%v actual=%v", publicSubnets, c.Etcd.Subnets)
					}

					for i, s := range c.PrivateSubnets() {
						if !s.ManageNATGateway() {
							t.Errorf("NAT gateway for the private subnet #%d should be created by kube-aws but it is not going to be", i)
						}

						if s.ManageRouteToInternet() {
							t.Errorf("Route to IGW shouldn't be created for a private subnet: %v", s)
						}
					}
				},
			},
		},
		{
			context: "WithNetworkTopologyControllerPrivateLB",
			configYaml: kubeAwsSettings.mainClusterYaml + `
vpcId: vpc-1a2b3c4d
# routeTableId must be omitted
# See https://github.com/coreos/kube-aws/pull/284#issuecomment-275962332
# routeTableId: rtb-1a2b3c4d
subnets:
- name: private1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.1.0/24"
  private: true
- name: private2
  availabilityZone: us-west-1b
  instanceCIDR: "10.0.2.0/24"
  private: true
- name: public1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.3.0/24"
- name: public2
  availabilityZone: us-west-1b
  instanceCIDR: "10.0.4.0/24"
controller:
  subnets:
  - name: private1
  - name: private2
  loadBalancer:
    private: true
etcd:
  subnets:
  - name: private1
  - name: private2
worker:
  nodePools:
  - name: pool1
    subnets:
    - name: public1
    - name: public2
`,
			assertConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				everyPublicSubnetHasRouteToIGW,
				hasTwoManagedNGWsAndEIPs,
				func(c *config.Config, t *testing.T) {
					private1 := model.NewPrivateSubnet("us-west-1a", "10.0.1.0/24")
					private1.Name = "private1"

					private2 := model.NewPrivateSubnet("us-west-1b", "10.0.2.0/24")
					private2.Name = "private2"

					public1 := model.NewPublicSubnet("us-west-1a", "10.0.3.0/24")
					public1.Name = "public1"

					public2 := model.NewPublicSubnet("us-west-1b", "10.0.4.0/24")
					public2.Name = "public2"

					subnets := []model.Subnet{
						private1,
						private2,
						public1,
						public2,
					}
					if !reflect.DeepEqual(c.AllSubnets(), subnets) {
						t.Errorf("Managed subnets didn't match: expected=%v actual=%v", subnets, c.AllSubnets())
					}

					importedPublicSubnets := []model.Subnet{
						model.NewPublicSubnetFromFn("us-west-1a", `{"Fn::ImportValue":{"Fn::Sub":"${ControlPlaneStackName}-Public1"}}`),
						model.NewPublicSubnetFromFn("us-west-1b", `{"Fn::ImportValue":{"Fn::Sub":"${ControlPlaneStackName}-Public2"}}`),
					}
					p := c.NodePools[0]
					if !reflect.DeepEqual(p.Subnets, importedPublicSubnets) {
						t.Errorf("Worker subnets didn't match: expected=%v actual=%v", importedPublicSubnets, p.Subnets)
					}

					privateSubnets := []model.Subnet{
						private1,
						private2,
					}
					if !reflect.DeepEqual(c.Controller.Subnets, privateSubnets) {
						t.Errorf("Controller subnets didn't match: expected=%v actual=%v", privateSubnets, c.Controller.Subnets)
					}
					if !reflect.DeepEqual(c.Controller.LoadBalancer.Subnets, privateSubnets) {
						t.Errorf("Controller loadbalancer subnets didn't match: expected=%v actual=%v", privateSubnets, c.Controller.LoadBalancer.Subnets)
					}
					if !reflect.DeepEqual(c.Etcd.Subnets, privateSubnets) {
						t.Errorf("Etcd subnets didn't match: expected=%v actual=%v", privateSubnets, c.Etcd.Subnets)
					}

					for i, s := range c.PrivateSubnets() {
						if !s.ManageNATGateway() {
							t.Errorf("NAT gateway for the private subnet #%d should be created by kube-aws but it is not going to be", i)
						}

						if s.ManageRouteToInternet() {
							t.Errorf("Route to IGW shouldn't be created for a private subnet: %v", s)
						}
					}
				},
			},
		},
		{
			context: "WithNetworkTopologyControllerPublicLB",
			configYaml: kubeAwsSettings.mainClusterYaml + `
vpcId: vpc-1a2b3c4d
# routeTableId must be omitted
# See https://github.com/coreos/kube-aws/pull/284#issuecomment-275962332
# routeTableId: rtb-1a2b3c4d
subnets:
- name: private1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.1.0/24"
  private: true
- name: private2
  availabilityZone: us-west-1b
  instanceCIDR: "10.0.2.0/24"
  private: true
- name: public1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.3.0/24"
- name: public2
  availabilityZone: us-west-1b
  instanceCIDR: "10.0.4.0/24"
controller:
  loadBalancer:
    private: false
etcd:
  subnets:
  - name: private1
  - name: private2
worker:
  nodePools:
  - name: pool1
    subnets:
    - name: public1
    - name: public2
`,
			assertConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				everyPublicSubnetHasRouteToIGW,
				hasTwoManagedNGWsAndEIPs,
				func(c *config.Config, t *testing.T) {
					private1 := model.NewPrivateSubnet("us-west-1a", "10.0.1.0/24")
					private1.Name = "private1"

					private2 := model.NewPrivateSubnet("us-west-1b", "10.0.2.0/24")
					private2.Name = "private2"

					public1 := model.NewPublicSubnet("us-west-1a", "10.0.3.0/24")
					public1.Name = "public1"

					public2 := model.NewPublicSubnet("us-west-1b", "10.0.4.0/24")
					public2.Name = "public2"

					subnets := []model.Subnet{
						private1,
						private2,
						public1,
						public2,
					}
					publicSubnets := []model.Subnet{
						public1,
						public2,
					}
					privateSubnets := []model.Subnet{
						private1,
						private2,
					}
					importedPublicSubnets := []model.Subnet{
						model.NewPublicSubnetFromFn("us-west-1a", `{"Fn::ImportValue":{"Fn::Sub":"${ControlPlaneStackName}-Public1"}}`),
						model.NewPublicSubnetFromFn("us-west-1b", `{"Fn::ImportValue":{"Fn::Sub":"${ControlPlaneStackName}-Public2"}}`),
					}

					if !reflect.DeepEqual(c.AllSubnets(), subnets) {
						t.Errorf("Managed subnets didn't match: expected=%v actual=%v", subnets, c.AllSubnets())
					}
					p := c.NodePools[0]
					if !reflect.DeepEqual(p.Subnets, importedPublicSubnets) {
						t.Errorf("Worker subnets didn't match: expected=%v actual=%v", importedPublicSubnets, p.Subnets)
					}
					if !reflect.DeepEqual(c.Controller.Subnets, publicSubnets) {
						t.Errorf("Controller subnets didn't match: expected=%v actual=%v", privateSubnets, c.Controller.Subnets)
					}
					if !reflect.DeepEqual(c.Controller.LoadBalancer.Subnets, publicSubnets) {
						t.Errorf("Controller loadbalancer subnets didn't match: expected=%v actual=%v", privateSubnets, c.Controller.LoadBalancer.Subnets)
					}
					if !reflect.DeepEqual(c.Etcd.Subnets, privateSubnets) {
						t.Errorf("Etcd subnets didn't match: expected=%v actual=%v", privateSubnets, c.Etcd.Subnets)
					}

					for i, s := range c.PrivateSubnets() {
						if !s.ManageNATGateway() {
							t.Errorf("NAT gateway for the private subnet #%d should be created by kube-aws but it is not going to be", i)
						}

						if s.ManageRouteToInternet() {
							t.Errorf("Route to IGW shouldn't be created for a private subnet: %v", s)
						}
					}
				},
			},
		},
		{
			context: "WithNetworkTopologyExistingSubnets",
			configYaml: kubeAwsSettings.mainClusterYaml + `
vpcId: vpc-1a2b3c4d
subnets:
- name: private1
  availabilityZone: us-west-1a
  id: subnet-1
  private: true
- name: private2
  availabilityZone: us-west-1b
  idFromStackOutput: mycluster-private-subnet-1
  private: true
- name: public1
  availabilityZone: us-west-1a
  id: subnet-2
- name: public2
  availabilityZone: us-west-1b
  idFromStackOutput: mycluster-public-subnet-1
controller:
  loadBalancer:
    private: false
etcd:
  subnets:
  - name: private1
  - name: private2
worker:
  nodePools:
  - name: pool1
    subnets:
    - name: public1
    - name: public2
`,
			assertConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				hasNoNGWsOrEIPsOrRoutes,
				func(c *config.Config, t *testing.T) {
					private1 := model.NewExistingPrivateSubnet("us-west-1a", "subnet-1")
					private1.Name = "private1"

					private2 := model.NewImportedPrivateSubnet("us-west-1b", "mycluster-private-subnet-1")
					private2.Name = "private2"

					public1 := model.NewExistingPublicSubnet("us-west-1a", "subnet-2")
					public1.Name = "public1"

					public2 := model.NewImportedPublicSubnet("us-west-1b", "mycluster-public-subnet-1")
					public2.Name = "public2"

					subnets := []model.Subnet{
						private1,
						private2,
						public1,
						public2,
					}
					publicSubnets := []model.Subnet{
						public1,
						public2,
					}
					privateSubnets := []model.Subnet{
						private1,
						private2,
					}

					if !reflect.DeepEqual(c.AllSubnets(), subnets) {
						t.Errorf("Managed subnets didn't match: expected=%v actual=%v", subnets, c.AllSubnets())
					}
					p := c.NodePools[0]
					if !reflect.DeepEqual(p.Subnets, publicSubnets) {
						t.Errorf("Worker subnets didn't match: expected=%v actual=%v", publicSubnets, p.Subnets)
					}
					if !reflect.DeepEqual(c.Controller.Subnets, publicSubnets) {
						t.Errorf("Controller subnets didn't match: expected=%v actual=%v", privateSubnets, c.Controller.Subnets)
					}
					if !reflect.DeepEqual(c.Controller.LoadBalancer.Subnets, publicSubnets) {
						t.Errorf("Controller loadbalancer subnets didn't match: expected=%v actual=%v", privateSubnets, c.Controller.LoadBalancer.Subnets)
					}
					if !reflect.DeepEqual(c.Etcd.Subnets, privateSubnets) {
						t.Errorf("Etcd subnets didn't match: expected=%v actual=%v", privateSubnets, c.Etcd.Subnets)
					}

					for i, s := range c.PrivateSubnets() {
						if s.ManageNATGateway() {
							t.Errorf("NAT gateway for the existing private subnet #%d should not be created by kube-aws", i)
						}

						if s.ManageRouteToInternet() {
							t.Errorf("Route to IGW shouldn't be created for a private subnet: %v", s)
						}
					}
				},
			},
		},
		{
			context: "WithNetworkTopologyExistingNATGateways",
			configYaml: kubeAwsSettings.mainClusterYaml + `
vpcId: vpc-1a2b3c4d
subnets:
- name: private1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.1.0/24"
  private: true
  natGateway:
    id: ngw-11111111
- name: private2
  availabilityZone: us-west-1b
  instanceCIDR: "10.0.2.0/24"
  private: true
  natGateway:
    id: ngw-22222222
- name: public1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.3.0/24"
- name: public2
  availabilityZone: us-west-1b
  instanceCIDR: "10.0.4.0/24"
etcd:
  subnets:
  - name: private1
  - name: private2
worker:
  nodePools:
  - name: pool1
    subnets:
    - name: public1
    - name: public2
`,
			assertConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				hasNoManagedNGWsButSpecificNumOfRoutesToUnmanagedNGWs(2),
				func(c *config.Config, t *testing.T) {
					private1 := model.NewPrivateSubnetWithPreconfiguredNATGateway("us-west-1a", "10.0.1.0/24", "ngw-11111111")
					private1.Name = "private1"

					private2 := model.NewPrivateSubnetWithPreconfiguredNATGateway("us-west-1b", "10.0.2.0/24", "ngw-22222222")
					private2.Name = "private2"

					public1 := model.NewPublicSubnet("us-west-1a", "10.0.3.0/24")
					public1.Name = "public1"

					public2 := model.NewPublicSubnet("us-west-1b", "10.0.4.0/24")
					public2.Name = "public2"

					subnets := []model.Subnet{
						private1,
						private2,
						public1,
						public2,
					}
					publicSubnets := []model.Subnet{
						public1,
						public2,
					}
					privateSubnets := []model.Subnet{
						private1,
						private2,
					}
					importedPublicSubnets := []model.Subnet{
						model.NewPublicSubnetFromFn("us-west-1a", `{"Fn::ImportValue":{"Fn::Sub":"${ControlPlaneStackName}-Public1"}}`),
						model.NewPublicSubnetFromFn("us-west-1b", `{"Fn::ImportValue":{"Fn::Sub":"${ControlPlaneStackName}-Public2"}}`),
					}

					if !reflect.DeepEqual(c.AllSubnets(), subnets) {
						t.Errorf("Managed subnets didn't match: expected=%v actual=%v", subnets, c.AllSubnets())
					}
					p := c.NodePools[0]
					if !reflect.DeepEqual(p.Subnets, importedPublicSubnets) {
						t.Errorf("Worker subnets didn't match: expected=%v actual=%v", importedPublicSubnets, p.Subnets)
					}
					if !reflect.DeepEqual(c.Controller.Subnets, publicSubnets) {
						t.Errorf("Controller subnets didn't match: expected=%v actual=%v", privateSubnets, c.Controller.Subnets)
					}
					if !reflect.DeepEqual(c.Controller.LoadBalancer.Subnets, publicSubnets) {
						t.Errorf("Controller loadbalancer subnets didn't match: expected=%v actual=%v", privateSubnets, c.Controller.LoadBalancer.Subnets)
					}
					if !reflect.DeepEqual(c.Etcd.Subnets, privateSubnets) {
						t.Errorf("Etcd subnets didn't match: expected=%v actual=%v", privateSubnets, c.Etcd.Subnets)
					}

					for i, s := range c.PrivateSubnets() {
						if s.ManageNATGateway() {
							t.Errorf("NAT gateway for the existing private subnet #%d should not be created by kube-aws", i)
						}

						if s.ManageRouteToInternet() {
							t.Errorf("Route to IGW shouldn't be created for a private subnet: %v", s)
						}
					}
				},
			},
		},
		{
			context: "WithNetworkTopologyExistingNATGatewayEIPs",
			configYaml: kubeAwsSettings.mainClusterYaml + `
vpcId: vpc-1a2b3c4d
subnets:
- name: private1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.1.0/24"
  private: true
  natGateway:
    eipAllocationId: eipalloc-11111111
- name: private2
  availabilityZone: us-west-1b
  instanceCIDR: "10.0.2.0/24"
  private: true
  natGateway:
    eipAllocationId: eipalloc-22222222
- name: public1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.3.0/24"
- name: public2
  availabilityZone: us-west-1b
  instanceCIDR: "10.0.4.0/24"
etcd:
  subnets:
  - name: private1
  - name: private2
worker:
  nodePools:
  - name: pool1
    subnets:
    - name: public1
    - name: public2
`,
			assertConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				hasSpecificNumOfManagedNGWsWithUnmanagedEIPs(2),
				hasPrivateSubnetsWithManagedNGWs(2),
				func(c *config.Config, t *testing.T) {
					private1 := model.NewPrivateSubnetWithPreconfiguredNATGatewayEIP("us-west-1a", "10.0.1.0/24", "eipalloc-11111111")
					private1.Name = "private1"

					private2 := model.NewPrivateSubnetWithPreconfiguredNATGatewayEIP("us-west-1b", "10.0.2.0/24", "eipalloc-22222222")
					private2.Name = "private2"

					public1 := model.NewPublicSubnet("us-west-1a", "10.0.3.0/24")
					public1.Name = "public1"

					public2 := model.NewPublicSubnet("us-west-1b", "10.0.4.0/24")
					public2.Name = "public2"

					subnets := []model.Subnet{
						private1,
						private2,
						public1,
						public2,
					}
					publicSubnets := []model.Subnet{
						public1,
						public2,
					}
					privateSubnets := []model.Subnet{
						private1,
						private2,
					}
					importedPublicSubnets := []model.Subnet{
						model.NewPublicSubnetFromFn("us-west-1a", `{"Fn::ImportValue":{"Fn::Sub":"${ControlPlaneStackName}-Public1"}}`),
						model.NewPublicSubnetFromFn("us-west-1b", `{"Fn::ImportValue":{"Fn::Sub":"${ControlPlaneStackName}-Public2"}}`),
					}

					if !reflect.DeepEqual(c.AllSubnets(), subnets) {
						t.Errorf("Managed subnets didn't match: expected=%v actual=%v", subnets, c.AllSubnets())
					}
					p := c.NodePools[0]
					if !reflect.DeepEqual(p.Subnets, importedPublicSubnets) {
						t.Errorf("Worker subnets didn't match: expected=%+v actual=%+v", importedPublicSubnets, p.Subnets)
					}
					if !reflect.DeepEqual(c.Controller.Subnets, publicSubnets) {
						t.Errorf("Controller subnets didn't match: expected=%v actual=%v", privateSubnets, c.Controller.Subnets)
					}
					if !reflect.DeepEqual(c.Controller.LoadBalancer.Subnets, publicSubnets) {
						t.Errorf("Controller loadbalancer subnets didn't match: expected=%v actual=%v", privateSubnets, c.Controller.LoadBalancer.Subnets)
					}
					if !reflect.DeepEqual(c.Etcd.Subnets, privateSubnets) {
						t.Errorf("Etcd subnets didn't match: expected=%v actual=%v", privateSubnets, c.Etcd.Subnets)
					}
				},
			},
		},
		{
			context: "WithSpotFleetEnabled",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    spotFleet:
      targetCapacity: 10
`,
			assertConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				hasDefaultLaunchSpecifications,
				spotFleetBasedNodePoolHasWaitSignalDisabled,
			},
		},
		{
			context: "WithSpotFleetWithCustomGp2RootVolumeSettings",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
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
			assertConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				spotFleetBasedNodePoolHasWaitSignalDisabled,
				func(c *config.Config, t *testing.T) {
					expected := []model.LaunchSpecification{
						{
							WeightedCapacity: 1,
							InstanceType:     "c4.large",
							SpotPrice:        "0.06",
							// RootVolumeSize was not specified in the configYaml but should default to workerRootVolumeSize * weightedCapacity
							// RootVolumeType was not specified in the configYaml but should default to "gp2"
							RootVolume: model.NewGp2RootVolume(40),
						},
						{
							WeightedCapacity: 2,
							InstanceType:     "c4.xlarge",
							SpotPrice:        "0.12",
							RootVolume:       model.NewGp2RootVolume(100),
						},
					}
					p := c.NodePools[0]
					actual := p.NodePoolConfig.SpotFleet.LaunchSpecifications
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
  nodePools:
  - name: pool1
    spotFleet:
      targetCapacity: 10
      unitRootVolumeSize: 40
      launchSpecifications:
      - weightedCapacity: 1
        instanceType: m4.large
      - weightedCapacity: 2
        instanceType: m4.xlarge
`,
			assertConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				spotFleetBasedNodePoolHasWaitSignalDisabled,
				func(c *config.Config, t *testing.T) {
					expected := []model.LaunchSpecification{
						{
							WeightedCapacity: 1,
							InstanceType:     "m4.large",
							SpotPrice:        "0.06",
							// RootVolumeType was not specified in the configYaml but should default to gp2:
							RootVolume: model.NewGp2RootVolume(40),
						},
						{
							WeightedCapacity: 2,
							InstanceType:     "m4.xlarge",
							SpotPrice:        "0.12",
							RootVolume:       model.NewGp2RootVolume(80),
						},
					}
					p := c.NodePools[0]
					actual := p.NodePoolConfig.SpotFleet.LaunchSpecifications
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
  nodePools:
  - name: pool1
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
			assertConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				spotFleetBasedNodePoolHasWaitSignalDisabled,
				func(c *config.Config, t *testing.T) {
					expected := []model.LaunchSpecification{
						{
							WeightedCapacity: 1,
							InstanceType:     "c4.large",
							SpotPrice:        "0.06",
							// RootVolumeSize was not specified in the configYaml but should default to workerRootVolumeSize * weightedCapacity
							// RootVolumeIOPS was not specified in the configYaml but should default to workerRootVolumeIOPS * weightedCapacity
							// RootVolumeType was not specified in the configYaml but should default to "io1"
							RootVolume: model.NewIo1RootVolume(40, 100),
						},
						{
							WeightedCapacity: 2,
							InstanceType:     "c4.xlarge",
							SpotPrice:        "0.12",
							// RootVolumeType was not specified in the configYaml but should default to:
							RootVolume: model.NewIo1RootVolume(80, 500),
						},
					}
					p := c.NodePools[0]
					actual := p.NodePoolConfig.SpotFleet.LaunchSpecifications
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
			context: "WithVpcIdSpecified",
			configYaml: minimalValidConfigYaml + `
vpcId: vpc-1a2b3c4d
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				hasDefaultExperimentalFeatures,
			},
		},
		{
			context: "WithVpcIdAndRouteTableIdSpecified",
			configYaml: minimalValidConfigYaml + `
vpcId: vpc-1a2b3c4d
routeTableId: rtb-1a2b3c4d
`,
			assertConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				func(c *config.Config, t *testing.T) {
					subnet1 := model.NewPublicSubnetWithPreconfiguredRouteTable("us-west-1c", "10.0.0.0/24", "rtb-1a2b3c4d")
					subnet1.Name = "Subnet0"
					subnets := []model.Subnet{
						subnet1,
					}
					expected := controlplane_config.EtcdSettings{
						Etcd: model.Etcd{
							Subnets: subnets,
						},
						EtcdCount:               1,
						EtcdInstanceType:        "t2.medium",
						EtcdRootVolumeSize:      30,
						EtcdRootVolumeType:      "gp2",
						EtcdDataVolumeSize:      30,
						EtcdDataVolumeType:      "gp2",
						EtcdDataVolumeEphemeral: false,
						EtcdTenancy:             "default",
					}
					actual := c.EtcdSettings
					if !reflect.DeepEqual(expected, actual) {
						t.Errorf(
							"EtcdSettings didn't match: expected=%v actual=%v",
							expected,
							actual,
						)
					}
				},
			},
		},
		{
			context: "WithWorkerManagedIamRoleName",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    managedIamRoleName: "yourManagedRole"
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				hasDefaultExperimentalFeatures,
				func(c *config.Config, t *testing.T) {
					if c.NodePools[0].ManagedIamRoleName != "yourManagedRole" {
						t.Errorf("managedIamRoleName: expected=yourManagedRole actual=%s", c.NodePools[0].ManagedIamRoleName)
					}
				},
			},
		},
		{
			context: "WithWorkerSecurityGroupIds",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    securityGroupIds:
    - sg-12345678
    - sg-abcdefab
    - sg-23456789
    - sg-bcdefabc
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				hasDefaultExperimentalFeatures,
				func(c *config.Config, t *testing.T) {
					p := c.NodePools[0]
					expectedWorkerSecurityGroupIds := []string{
						`sg-12345678`, `sg-abcdefab`, `sg-23456789`, `sg-bcdefabc`,
					}
					if !reflect.DeepEqual(p.SecurityGroupIds, expectedWorkerSecurityGroupIds) {
						t.Errorf("WorkerSecurityGroupIds didn't match: expected=%v actual=%v", expectedWorkerSecurityGroupIds, p.SecurityGroupIds)
					}

					expectedWorkerSecurityGroupRefs := []string{
						`"sg-12345678"`, `"sg-abcdefab"`, `"sg-23456789"`, `"sg-bcdefabc"`,
						`{"Fn::ImportValue" : {"Fn::Sub" : "${ControlPlaneStackName}-WorkerSecurityGroup"}}`,
					}
					if !reflect.DeepEqual(p.SecurityGroupRefs(), expectedWorkerSecurityGroupRefs) {
						t.Errorf("SecurityGroupRefs didn't match: expected=%v actual=%v", expectedWorkerSecurityGroupRefs, p.SecurityGroupRefs())
					}
				},
			},
		},
		{
			context: "WithWorkerAndLBSecurityGroupIds",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    securityGroupIds:
    - sg-12345678
    - sg-abcdefab
    loadBalancer:
      enabled: true
      securityGroupIds:
        - sg-23456789
        - sg-bcdefabc
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				func(c *config.Config, t *testing.T) {
					p := c.NodePools[0]
					expectedWorkerSecurityGroupIds := []string{
						`sg-12345678`, `sg-abcdefab`,
					}
					if !reflect.DeepEqual(p.SecurityGroupIds, expectedWorkerSecurityGroupIds) {
						t.Errorf("WorkerSecurityGroupIds didn't match: expected=%v actual=%v", expectedWorkerSecurityGroupIds, p.SecurityGroupIds)
					}

					expectedLBSecurityGroupIds := []string{
						`sg-23456789`, `sg-bcdefabc`,
					}
					if !reflect.DeepEqual(p.LoadBalancer.SecurityGroupIds, expectedLBSecurityGroupIds) {
						t.Errorf("LBSecurityGroupIds didn't match: expected=%v actual=%v", expectedLBSecurityGroupIds, p.LoadBalancer.SecurityGroupIds)
					}

					expectedWorkerSecurityGroupRefs := []string{
						`"sg-23456789"`, `"sg-bcdefabc"`, `"sg-12345678"`, `"sg-abcdefab"`,
						`{"Fn::ImportValue" : {"Fn::Sub" : "${ControlPlaneStackName}-WorkerSecurityGroup"}}`,
					}
					if !reflect.DeepEqual(p.SecurityGroupRefs(), expectedWorkerSecurityGroupRefs) {
						t.Errorf("SecurityGroupRefs didn't match: expected=%v actual=%v", expectedWorkerSecurityGroupRefs, p.SecurityGroupRefs())
					}
				},
			},
		},
		{
			context: "WithWorkerAndALBSecurityGroupIds",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    securityGroupIds:
    - sg-12345678
    - sg-abcdefab
    targetGroup:
      enabled: true
      securityGroupIds:
        - sg-23456789
        - sg-bcdefabc
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				func(c *config.Config, t *testing.T) {
					p := c.NodePools[0]
					expectedWorkerSecurityGroupIds := []string{
						`sg-12345678`, `sg-abcdefab`,
					}
					if !reflect.DeepEqual(p.SecurityGroupIds, expectedWorkerSecurityGroupIds) {
						t.Errorf("WorkerSecurityGroupIds didn't match: expected=%v actual=%v", expectedWorkerSecurityGroupIds, p.SecurityGroupIds)
					}

					expectedALBSecurityGroupIds := []string{
						`sg-23456789`, `sg-bcdefabc`,
					}
					if !reflect.DeepEqual(p.TargetGroup.SecurityGroupIds, expectedALBSecurityGroupIds) {
						t.Errorf("LBSecurityGroupIds didn't match: expected=%v actual=%v", expectedALBSecurityGroupIds, p.TargetGroup.SecurityGroupIds)
					}

					expectedWorkerSecurityGroupRefs := []string{
						`"sg-23456789"`, `"sg-bcdefabc"`, `"sg-12345678"`, `"sg-abcdefab"`,
						`{"Fn::ImportValue" : {"Fn::Sub" : "${ControlPlaneStackName}-WorkerSecurityGroup"}}`,
					}
					if !reflect.DeepEqual(p.SecurityGroupRefs(), expectedWorkerSecurityGroupRefs) {
						t.Errorf("SecurityGroupRefs didn't match: expected=%v actual=%v", expectedWorkerSecurityGroupRefs, p.SecurityGroupRefs())
					}
				},
			},
		},
		{
			context: "WithDedicatedInstanceTenancy",
			configYaml: minimalValidConfigYaml + `
workerTenancy: dedicated
controllerTenancy: dedicated
etcdTenancy: dedicated
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					if c.EtcdSettings.EtcdTenancy != "dedicated" {
						t.Errorf("EtcdSettings.EtcdTenancy didn't match: expected=dedicated actual=%s", c.EtcdSettings.EtcdTenancy)
					}
					if c.WorkerTenancy != "dedicated" {
						t.Errorf("WorkerTenancy didn't match: expected=dedicated actual=%s", c.WorkerTenancy)
					}
					if c.ControllerTenancy != "dedicated" {
						t.Errorf("ControllerTenancy didn't match: expected=dedicated actual=%s", c.ControllerTenancy)
					}
				},
			},
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
			assertConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				func(c *config.Config, t *testing.T) {
					subnet1 := model.NewPublicSubnetWithPreconfiguredRouteTable("us-west-1c", "10.0.0.0/24", "rtb-1a2b3c4d")
					subnet1.Name = "Subnet0"
					subnets := []model.Subnet{
						subnet1,
					}
					expected := controlplane_config.EtcdSettings{
						Etcd: model.Etcd{
							Subnets: subnets,
						},
						EtcdCount:               2,
						EtcdInstanceType:        "t2.medium",
						EtcdRootVolumeSize:      101,
						EtcdRootVolumeType:      "io1",
						EtcdRootVolumeIOPS:      102,
						EtcdDataVolumeSize:      103,
						EtcdDataVolumeType:      "io1",
						EtcdDataVolumeIOPS:      104,
						EtcdDataVolumeEphemeral: false,
						EtcdTenancy:             "default",
					}
					actual := c.EtcdSettings
					if !reflect.DeepEqual(expected, actual) {
						t.Errorf(
							"EtcdSettings didn't match: expected=%v actual=%v",
							expected,
							actual,
						)
					}
				},
			},
		},
	}

	for _, validCase := range validCases {
		t.Run(validCase.context, func(t *testing.T) {
			configBytes := validCase.configYaml
			providedConfig, err := config.ConfigFromBytesWithEncryptService([]byte(configBytes), helper.DummyEncryptService{})
			if err != nil {
				t.Errorf("failed to parse config %s: %v", configBytes, err)
				t.FailNow()
			}

			t.Run("AssertConfig", func(t *testing.T) {
				for _, assertion := range validCase.assertConfig {
					assertion(providedConfig, t)
				}
			})

			helper.WithDummyCredentials(func(dummyTlsAssetsDir string) {
				var stackTemplateOptions = root.NewOptions(s3URI, false, false)
				stackTemplateOptions.TLSAssetsDir = dummyTlsAssetsDir
				stackTemplateOptions.ControllerTmplFile = "../../core/controlplane/config/templates/cloud-config-controller"
				stackTemplateOptions.WorkerTmplFile = "../../core/controlplane/config/templates/cloud-config-worker"
				stackTemplateOptions.EtcdTmplFile = "../../core/controlplane/config/templates/cloud-config-etcd"
				stackTemplateOptions.RootStackTemplateTmplFile = "../../core/root/config/templates/stack-template.json"
				stackTemplateOptions.NodePoolStackTemplateTmplFile = "../../core/nodepool/config/templates/stack-template.json"
				stackTemplateOptions.ControlPlaneStackTemplateTmplFile = "../../core/controlplane/config/templates/stack-template.json"

				cluster, err := root.ClusterFromConfig(providedConfig, stackTemplateOptions, false)
				if err != nil {
					t.Errorf("failed to create cluster driver : %v", err)
					t.FailNow()
				}

				t.Run("AssertCluster", func(t *testing.T) {
					for _, assertion := range validCase.assertCluster {
						assertion(cluster, t)
					}
				})

				t.Run("ValidateUserData", func(t *testing.T) {
					if err := cluster.ValidateUserData(); err != nil {
						t.Errorf("failed to validate user data: %v", err)
					}
				})

				t.Run("ValidateTemplates", func(t *testing.T) {
					if err := cluster.ValidateTemplates(); err != nil {
						t.Errorf("failed to render stack template: %v", err)
					}
				})

				if os.Getenv("KUBE_AWS_INTEGRATION_TEST") == "" {
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
			context: "WithClusterAutoscalerEnabledForControlPlane",
			configYaml: minimalValidConfigYaml + `
controller:
  clusterAutoscaler:
    minSize: 1
    maxSize: 10
`,
			expectedErrorMessage: "cluster-autoscaler can't be enabled for a control plane because " +
				"allowing so for a group of controller nodes spreading over 2 or more availability zones " +
				"results in unreliability while scaling nodes out.",
		},
		{
			context: "WithInvalidTaint",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    taints:
    - key: foo
      value: bar
      effect: UnknownEffect
`,
			expectedErrorMessage: "Effect must be NoSchdule or PreferNoSchedule, but was UnknownEffect",
		},
		{
			context: "WithAwsNodeLabelEnabledForTooLongClusterNameAndPoolName",
			configYaml: minimalValidConfigYaml + `
# clusterName + nodePools[].name should be less than or equal to 25 characters or the launch configuration name
# "mykubeawsclustername-mynestedstackname-1N2C4K3LLBEDZ-ControllersLC-BC2S9P3JG2QD" exceeds the limit of 63 characters
# See https://kubernetes.io/docs/user-guide/labels/#syntax-and-character-set
clusterName: my_cluster1 # 11 characters
worker:
  nodePools:
  - name: workernodepool1 # 15 characters
    awsNodeLabels:
      enabled: true
`,
			expectedErrorMessage: "awsNodeLabels can't be enabled for node pool because the total number of characters in clusterName(=\"my_cluster1\") + node pool's name(=\"workernodepool1\") exceeds the limit of 25",
		},
		{
			context: "WithAwsNodeLabelEnabledForTooLongClusterName",
			configYaml: minimalValidConfigYaml + `
# clusterName should be less than or equal to 21 characters or the launch configuration name
# "mykubeawsclustername-mynestedstackname-1N2C4K3LLBEDZ-ControllersLC-BC2S9P3JG2QD" exceeds the limit of 63 characters
# See https://kubernetes.io/docs/user-guide/labels/#syntax-and-character-set
clusterName: my_long_long_cluster_1 # 22 characters
experimental:
  awsNodeLabels:
     enabled: true
`,
			expectedErrorMessage: "awsNodeLabels can't be enabled for controllers because the total number of characters in clusterName(=\"my_long_long_cluster_1\") exceeds the limit of 21",
		},
		{
			context: "WithNonZeroWorkerCount",
			configYaml: minimalValidConfigYaml + `
workerCount: 1
`,
			expectedErrorMessage: "`workerCount` is removed. Set worker.nodePools[].count per node pool instead",
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
			context: "WithRouteTableIdSpecified",
			configYaml: minimalValidConfigYaml + `
# vpcId must be specified if routeTableId is specified
routeTableId: rtb-1a2b3c4d
`,
		},
		{
			context: "WithWorkerSecurityGroupIds",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
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
worker:
  nodePools:
  - name: pool1
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
worker:
  nodePools:
  - name: pool1
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
	}

	for _, invalidCase := range parseErrorCases {
		t.Run(invalidCase.context, func(t *testing.T) {
			configBytes := invalidCase.configYaml
			providedConfig, err := config.ConfigFromBytes([]byte(configBytes))
			if err == nil {
				t.Errorf("expected to fail parsing config %s: %+v", configBytes, *providedConfig)
				t.FailNow()
			}

			errorMsg := fmt.Sprintf("%v", err)
			if !strings.Contains(errorMsg, invalidCase.expectedErrorMessage) {
				t.Errorf(`expected "%s" to be contained in the errror message : %s`, invalidCase.expectedErrorMessage, errorMsg)
			}
		})
	}
}
