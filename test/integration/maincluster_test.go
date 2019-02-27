package integration

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kubernetes-incubator/kube-aws/builtin"
	"github.com/kubernetes-incubator/kube-aws/cfnstack"
	"github.com/kubernetes-incubator/kube-aws/core/root"
	"github.com/kubernetes-incubator/kube-aws/core/root/config"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"github.com/kubernetes-incubator/kube-aws/pkg/model"
	"github.com/kubernetes-incubator/kube-aws/test/helper"
)

type ConfigTester func(c *config.Config, t *testing.T)
type ClusterTester func(c *root.Cluster, t *testing.T)

// Integration testing with real AWS services including S3, KMS, CloudFormation
func TestMainClusterConfig(t *testing.T) {
	kubeAwsSettings := newKubeAwsSettingsFromEnv(t)

	s3URI, s3URIExists := os.LookupEnv("KUBE_AWS_S3_DIR_URI")

	if !s3URIExists || s3URI == "" {
		s3URI = "s3://mybucket/mydir"
		t.Logf(`Falling back s3URI to a stub value "%s" for tests of validating stack templates. No assets will actually be uploaded to S3`, s3URI)
	} else {
		log.Printf("s3URI is %s", s3URI)
	}

	s3Loc, err := cfnstack.S3URIFromString(s3URI)
	if err != nil {
		t.Errorf("failed to parse s3 uri: %v", err)
		t.FailNow()
	}
	s3Bucket := s3Loc.Bucket()
	s3Dir := s3Loc.KeyComponents()[0]

	firstAz := kubeAwsSettings.region + "c"

	hasDefaultEtcdSettings := func(c *config.Config, t *testing.T) {
		subnet1 := api.NewPublicSubnet(firstAz, "10.0.0.0/24")
		subnet1.Name = "Subnet0"
		expected := api.EtcdSettings{
			Etcd: api.Etcd{
				EC2Instance: api.EC2Instance{
					Count:        1,
					InstanceType: "t2.medium",
					Tenancy:      "default",
					RootVolume: api.RootVolume{
						Size: 30,
						Type: "gp2",
						IOPS: 0,
					},
				},
				DataVolume: api.DataVolume{
					Size:      30,
					Type:      "gp2",
					IOPS:      0,
					Ephemeral: false,
				},
				Subnets: api.Subnets{
					subnet1,
				},
				UserSuppliedArgs: api.UserSuppliedArgs{
					QuotaBackendBytes: api.DefaultQuotaBackendBytes,
				},
			},
		}
		actual := c.EtcdSettings
		if diff := cmp.Diff(actual, expected); diff != "" {
			t.Errorf("EtcdSettings didn't match: %s", diff)
		}
	}

	hasDefaultExperimentalFeatures := func(c *config.Config, t *testing.T) {
		expected := api.Experimental{
			Admission: api.Admission{
				PodSecurityPolicy: api.PodSecurityPolicy{
					Enabled: false,
				},
				AlwaysPullImages: api.AlwaysPullImages{
					Enabled: false,
				},
				DenyEscalatingExec: api.DenyEscalatingExec{
					Enabled: false,
				},
				Priority: api.Priority{
					Enabled: false,
				},
				MutatingAdmissionWebhook: api.MutatingAdmissionWebhook{
					Enabled: false,
				},
				ValidatingAdmissionWebhook: api.ValidatingAdmissionWebhook{
					Enabled: false,
				},
				PersistentVolumeClaimResize: api.PersistentVolumeClaimResize{
					Enabled: false,
				},
			},
			AuditLog: api.AuditLog{
				Enabled:   false,
				LogPath:   "/var/log/kube-apiserver-audit.log",
				MaxAge:    30,
				MaxBackup: 1,
				MaxSize:   100,
			},
			Authentication: api.Authentication{
				Webhook: api.Webhook{
					Enabled:  false,
					CacheTTL: "5m0s",
					Config:   "",
				},
			},
			AwsEnvironment: api.AwsEnvironment{
				Enabled: false,
			},
			AwsNodeLabels: api.AwsNodeLabels{
				Enabled: false,
			},
			ClusterAutoscalerSupport: api.ClusterAutoscalerSupport{
				Enabled: true,
				Options: map[string]string{},
			},
			TLSBootstrap: api.TLSBootstrap{
				Enabled: false,
			},
			EphemeralImageStorage: api.EphemeralImageStorage{
				Enabled:    false,
				Disk:       "xvdb",
				Filesystem: "xfs",
			},
			KIAMSupport: api.KIAMSupport{
				Enabled:         false,
				Image:           api.Image{Repo: "quay.io/uswitch/kiam", Tag: "v2.8", RktPullDocker: false},
				SessionDuration: "15m",
				ServerAddresses: api.KIAMServerAddresses{ServerAddress: "localhost:443", AgentAddress: "kiam-server:443"},
			},
			Kube2IamSupport: api.Kube2IamSupport{
				Enabled: false,
			},
			GpuSupport: api.GpuSupport{
				Enabled:      false,
				Version:      "",
				InstallImage: "shelmangroup/coreos-nvidia-driver-installer:latest",
			},
			LoadBalancer: api.LoadBalancer{
				Enabled: false,
			},
			Oidc: api.Oidc{
				Enabled:       false,
				IssuerUrl:     "https://accounts.google.com",
				ClientId:      "kubernetes",
				UsernameClaim: "email",
				GroupsClaim:   "groups",
			},
			NodeDrainer: api.NodeDrainer{
				Enabled:      false,
				DrainTimeout: 5,
			},
		}

		actual := c.Experimental

		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("experimental settings didn't match :\nexpected=%v\nactual=%v", expected, actual)
		}

		if !c.WaitSignal.Enabled() {
			t.Errorf("waitSignal should be enabled but was not: %v", c.WaitSignal)
		}

		if c.WaitSignal.MaxBatchSize() != 1 {
			t.Errorf("waitSignal.maxBatchSize should be 1 but was %d: %v", c.WaitSignal.MaxBatchSize(), c.WaitSignal)
		}

		if len(c.NodePools) > 0 && c.NodePools[0].ClusterAutoscalerSupport.Enabled {
			t.Errorf("ClusterAutoscalerSupport must be disabled by default on node pools")
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
		expected := []api.LaunchSpecification{
			{
				WeightedCapacity: 1,
				InstanceType:     "c4.large",
				SpotPrice:        "",
				RootVolume:       api.NewGp2RootVolume(30),
			},
			{
				WeightedCapacity: 2,
				InstanceType:     "c4.xlarge",
				SpotPrice:        "",
				RootVolume:       api.NewGp2RootVolume(60),
			},
		}
		p := c.NodePools[0]
		actual := p.WorkerNodePool.SpotFleet.LaunchSpecifications
		if !reflect.DeepEqual(expected, actual) {
			t.Errorf(
				"LaunchSpecifications didn't match: expected=%v actual=%v",
				expected,
				actual,
			)
		}

		globalSpotPrice := p.WorkerNodePool.SpotFleet.SpotPrice
		if globalSpotPrice != "0.06" {
			t.Errorf("Default spot price is expected to be 0.06 but was: %s", globalSpotPrice)
		}
	}

	spotFleetBasedNodePoolHasWaitSignalDisabled := func(c *config.Config, t *testing.T) {
		p := c.NodePools[0]

		if !p.SpotFleet.Enabled() {
			t.Errorf("1st node pool is expected to be a spot fleet based one but was not: %+v", p)
		}

		if p.WaitSignal.Enabled() {
			t.Errorf(
				"WaitSignal should be enabled but was not: %v",
				p.WaitSignal,
			)
		}
	}

	asgBasedNodePoolHasWaitSignalEnabled := func(c *config.Config, t *testing.T) {
		p := c.NodePools[0]

		if p.SpotFleet.Enabled() {
			t.Errorf("1st node pool is expected to be an asg-based one but was not: %+v", p)
		}

		if !p.WaitSignal.Enabled() {
			t.Errorf(
				"WaitSignal should be disabled but was not: %v",
				p.WaitSignal,
			)
		}
	}

	hasDefaultNodePoolRollingStrategy := func(c *config.Config, t *testing.T) {
		s := c.NodePools[0].NodePoolRollingStrategy

		if s != "Parallel" {
			t.Errorf("Default nodePool rolling strategy should be 'Parallel' but is not: %v", s)
		}
	}

	hasSpecificNodePoolRollingStrategy := func(expRollingStrategy string) func(c *config.Config, t *testing.T) {
		return func(c *config.Config, t *testing.T) {
			actRollingStrategy := c.NodePools[0].NodePoolRollingStrategy
			if actRollingStrategy != expRollingStrategy {
				t.Errorf("The nodePool Rolling Strategy (%s) does not match with the expected one: %s", actRollingStrategy, expRollingStrategy)
			}
		}
	}

	hasWorkerAndNodePoolStrategy := func(expWorkerStrategy, expNodePoolStrategy string) func(c *config.Config, t *testing.T) {
		return func(c *config.Config, t *testing.T) {
			actWorkerStrategy := c.NodePools[0].NodePoolRollingStrategy
			actNodePoolStrategy := c.NodePools[1].NodePoolRollingStrategy

			if expWorkerStrategy != actWorkerStrategy {
				t.Errorf("The nodePool Rolling Strategy (%s) does not match with the expected one: %s", actWorkerStrategy, expWorkerStrategy)
			}
			if expNodePoolStrategy != actNodePoolStrategy {
				t.Errorf("The nodePool Rolling Strategy (%s) does not match with the expected one: %s", actNodePoolStrategy, expNodePoolStrategy)
			}
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
					t.Errorf("NGW #%d is expected to be unmanaged by kube-aws but was not: %+v", i, n)
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

	hasDefaultCluster := func(c *root.Cluster, t *testing.T) {
		assets, err := c.EnsureAllAssetsGenerated()
		if err != nil {
			t.Errorf("failed to list assets: %v", err)
			t.FailNow()
		}

		t.Run("Assets/RootStackTemplate", func(t *testing.T) {
			cluster := kubeAwsSettings.clusterName
			stack := kubeAwsSettings.clusterName
			file := "stack.json"
			expected := api.Asset{
				Content: "",
				AssetLocation: api.AssetLocation{
					ID:     api.NewAssetID(stack, file),
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
			expected := api.Asset{
				Content: builtin.String("stack-templates/control-plane.json.tmpl"),
				AssetLocation: api.AssetLocation{
					ID:     api.NewAssetID(stack, file),
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

	mainClusterYaml := kubeAwsSettings.mainClusterYaml()
	minimalValidConfigYaml := kubeAwsSettings.minimumValidClusterYamlWithAZ("c")
	configYamlWithoutExernalDNSName := kubeAwsSettings.mainClusterYamlWithoutAPIEndpoint() + `
availabilityZone: us-west-1c
`

	validCases := []struct {
		context       string
		configYaml    string
		assertConfig  []ConfigTester
		assertCluster []ClusterTester
	}{
		{
			context: "WithAddons",
			configYaml: minimalValidConfigYaml + `
addons:
  rescheduler:
    enabled: true
  clusterAutoscaler:
    enabled: true
    options:
      v: 5
      test: present
  metricsServer:
    enabled: true
worker:
  nodePools:
  - name: pool1
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				asgBasedNodePoolHasWaitSignalEnabled,
				func(c *config.Config, t *testing.T) {
					expected := api.Addons{
						Rescheduler: api.Rescheduler{
							Enabled: true,
						},
						ClusterAutoscaler: api.ClusterAutoscalerSupport{
							Enabled: true,
							Options: map[string]string{"v": "5", "test": "present"},
						},
						MetricsServer: api.MetricsServer{
							Enabled: true,
						},
						APIServerAggregator: api.APIServerAggregator{
							Enabled: true,
						},
					}

					actual := c.Addons

					if !reflect.DeepEqual(expected, actual) {
						t.Errorf("addons didn't match : expected=%+v actual=%+v", expected, actual)
					}
				},
			},
			assertCluster: []ClusterTester{
				hasDefaultCluster,
			},
		},
		{
			context: "WithAutoscalingByClusterAutoscaler",
			configYaml: minimalValidConfigYaml + `
addons:
  clusterAutoscaler:
    enabled: true
worker:
  nodePools:
  - name: pool1
    autoscaling:
      clusterAutoscaler:
        enabled: true
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				asgBasedNodePoolHasWaitSignalEnabled,
				func(c *config.Config, t *testing.T) {
					p := c.NodePools[0]

					expected := true
					actual := p.Autoscaling.ClusterAutoscaler.Enabled
					if !reflect.DeepEqual(expected, actual) {
						t.Errorf("autoscaling.clusterAutoscaler.enabled didn't match : expected=%v actual=%v", expected, actual)
					}
				},
			},
			assertCluster: []ClusterTester{
				hasDefaultCluster,
			},
		},
		{
			context: "WithAPIEndpointLBAPIAccessAllowedSourceCIDRsSpecified",
			configYaml: configYamlWithoutExernalDNSName + `
apiEndpoints:
- name: default
  dnsName: k8s.example.com
  loadBalancer:
    apiAccessAllowedSourceCIDRs:
    - 1.2.3.255/32
    hostedZone:
      id: a1b2c4
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					l := len(c.APIEndpointConfigs[0].LoadBalancer.APIAccessAllowedSourceCIDRs)
					if l != 1 {
						t.Errorf("unexpected size of apiEndpoints[0].loadBalancer.apiAccessAllowedSourceCIDRs: %d", l)
						t.FailNow()
					}
					actual := c.APIEndpointConfigs[0].LoadBalancer.APIAccessAllowedSourceCIDRs[0].String()
					expected := "1.2.3.255/32"
					if actual != expected {
						t.Errorf("unexpected cidr in apiEndpoints[0].loadBalancer.apiAccessAllowedSourceCIDRs[0]. expected = %s, actual = %s", expected, actual)
					}
				},
			},
		},
		{
			context: "WithAPIEndpointLBAPIAccessAllowedSourceCIDRsOmitted",
			configYaml: configYamlWithoutExernalDNSName + `
apiEndpoints:
- name: default
  dnsName: k8s.example.com
  loadBalancer:
    hostedZone:
      id: a1b2c4
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					l := len(c.APIEndpointConfigs[0].LoadBalancer.APIAccessAllowedSourceCIDRs)
					if l != 1 {
						t.Errorf("unexpected size of apiEndpoints[0].loadBalancer.apiAccessAllowedSourceCIDRs: %d", l)
						t.FailNow()
					}
					actual := c.APIEndpointConfigs[0].LoadBalancer.APIAccessAllowedSourceCIDRs[0].String()
					expected := "0.0.0.0/0"
					if actual != expected {
						t.Errorf("unexpected cidr in apiEndpoints[0].loadBalancer.apiAccessAllowedSourceCIDRs[0]. expected = %s, actual = %s", expected, actual)
					}
				},
			},
		},
		{
			context:    "WithKubeProxyIPVSModeDisabledByDefault",
			configYaml: minimalValidConfigYaml,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					if c.KubeProxy.IPVSMode.Enabled != false {
						t.Errorf("kube-proxy IPVS mode must be disabled by default")
					}

					expectedScheduler := "rr"
					if c.KubeProxy.IPVSMode.Scheduler != expectedScheduler {
						t.Errorf("IPVS scheduler should be by default set to: %s (actual = %s)", expectedScheduler, c.KubeProxy.IPVSMode.Scheduler)
					}

					expectedSyncPeriod := "60s"
					if c.KubeProxy.IPVSMode.SyncPeriod != expectedSyncPeriod {
						t.Errorf("Sync period should be by default set to: %s (actual = %s)", expectedSyncPeriod, c.KubeProxy.IPVSMode.SyncPeriod)
					}

					expectedMinSyncPeriod := "10s"
					if c.KubeProxy.IPVSMode.MinSyncPeriod != expectedMinSyncPeriod {
						t.Errorf("Minimal sync period should be by default set to: %s (actual = %s)", expectedMinSyncPeriod, c.KubeProxy.IPVSMode.MinSyncPeriod)
					}
				},
			},
		},
		{
			context: "WithKubeProxyIPVSModeEnabled",
			configYaml: minimalValidConfigYaml + `
kubeProxy:
  ipvsMode:
    enabled: true
    scheduler: lc
    syncPeriod: 90s
    minSyncPeriod: 15s
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					if c.KubeProxy.IPVSMode.Enabled != true {
						t.Errorf("kube-proxy IPVS mode must be enabled")
					}

					expectedScheduler := "lc"
					if c.KubeProxy.IPVSMode.Scheduler != expectedScheduler {
						t.Errorf("IPVS scheduler should be set to: %s (actual = %s)", expectedScheduler, c.KubeProxy.IPVSMode.Scheduler)
					}

					expectedSyncPeriod := "90s"
					if c.KubeProxy.IPVSMode.SyncPeriod != expectedSyncPeriod {
						t.Errorf("Sync period should be set to: %s (actual = %s)", expectedSyncPeriod, c.KubeProxy.IPVSMode.SyncPeriod)
					}

					expectedMinSyncPeriod := "15s"
					if c.KubeProxy.IPVSMode.MinSyncPeriod != expectedMinSyncPeriod {
						t.Errorf("Minimal sync period should be set to: %s (actual = %s)", expectedMinSyncPeriod, c.KubeProxy.IPVSMode.MinSyncPeriod)
					}
				},
			},
		},
		{
			// See https://github.com/kubernetes-incubator/kube-aws/issues/365
			context:    "WithClusterNameContainsHyphens",
			configYaml: kubeAwsSettings.withClusterName("my-cluster").minimumValidClusterYaml(),
		},
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
			context: "WithDifferentReleaseChannels",
			configYaml: minimalValidConfigYaml + `
releaseChannel: stable
worker:
  nodePools:
  - name: pool1
    releaseChannel: alpha
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				asgBasedNodePoolHasWaitSignalEnabled,
			},
			assertCluster: []ClusterTester{
				func(c *root.Cluster, t *testing.T) {
					cp := c.ControlPlane().Config.AMI
					np := c.NodePools()[0].NodePoolConfig.AMI

					if cp == "" {
						t.Error("the default AMI ID should not be empty but it was")
					}

					if np == "" {
						t.Error("the AMI ID for the node pool should not be empty but it was")
					}

					if cp == np {
						t.Errorf("the default AMI ID and the AMI ID for the node pool should not match but they did: default=%s, nodepool=%s", cp, np)
					}
				},
			},
		},
		{
			context: "WithElasticFileSystemId",
			configYaml: minimalValidConfigYaml + `
elasticFileSystemId: efs-12345
worker:
  nodePools:
  - name: pool1
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					if c.NodePools[0].ElasticFileSystemID != "efs-12345" {
						t.Errorf("The value of worker.nodePools[0].elasticFileSystemId should match the one for the top-leve elasticFileSystemId, but it wan't: worker.nodePools[0].elasticFileSystemId=%s", c.NodePools[0].ElasticFileSystemID)
					}
				},
			},
		},
		{
			context: "WithElasticFileSystemIdInSpecificNodePool",
			configYaml: mainClusterYaml + `
subnets:
- name: existing1
  id: subnet-12345
  availabilityZone: us-west-1a
worker:
  nodePools:
  - name: pool1
    subnets:
    - name: existing1
    elasticFileSystemId: efs-12345
  - name: pool2
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					if c.NodePools[0].ElasticFileSystemID != "efs-12345" {
						t.Errorf("Unexpected worker.nodePools[0].elasticFileSystemId: %s", c.NodePools[0].ElasticFileSystemID)
					}
					if c.NodePools[1].ElasticFileSystemID != "" {
						t.Errorf("Unexpected worker.nodePools[1].elasticFileSystemId: %s", c.NodePools[1].ElasticFileSystemID)
					}
				},
			},
		},
		{
			context: "WithEtcdDataVolumeEncrypted",
			configYaml: minimalValidConfigYaml + `
etcd:
  dataVolume:
    encrypted: true
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					if !c.Etcd.DataVolume.Encrypted {
						t.Errorf("Etcd data volume should be encrypted but was not: %v", c.Etcd)
					}
				},
			},
		},
		{
			context: "WithEtcdDataVolumeEncryptedKMSKeyARN",
			configYaml: minimalValidConfigYaml + `
etcd:
  dataVolume:
    encrypted: true
  kmsKeyArn: arn:aws:kms:eu-west-1:XXX:key/XXX
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					expected := "arn:aws:kms:eu-west-1:XXX:key/XXX"
					if c.Etcd.KMSKeyARN() != expected {
						t.Errorf("Etcd data volume KMS Key ARN didn't match : expected=%v actual=%v", expected, c.Etcd.KMSKeyARN())
					}
					if !c.Etcd.DataVolume.Encrypted {
						t.Error("Etcd data volume should be encrypted but was not")
					}
				},
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
					subnet1 := api.NewPublicSubnet(firstAz, "10.0.0.0/24")
					subnet1.Name = "Subnet0"
					expected := api.EtcdSettings{
						Etcd: api.Etcd{
							Cluster: api.EtcdCluster{
								MemberIdentityProvider: "eip",
							},
							EC2Instance: api.EC2Instance{
								Count:        1,
								InstanceType: "t2.medium",
								Tenancy:      "default",
								RootVolume: api.RootVolume{
									Size: 30,
									Type: "gp2",
									IOPS: 0,
								},
							},
							DataVolume: api.DataVolume{
								Size:      30,
								Type:      "gp2",
								IOPS:      0,
								Ephemeral: false,
							},
							Subnets: api.Subnets{
								subnet1,
							},
							UserSuppliedArgs: api.UserSuppliedArgs{
								QuotaBackendBytes: api.DefaultQuotaBackendBytes,
							},
						},
					}
					actual := c.EtcdSettings
					if diff := cmp.Diff(actual, expected); diff != "" {
						t.Errorf("EtcdSettings didn't match: %s", diff)
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
					subnet1 := api.NewPublicSubnet(firstAz, "10.0.0.0/24")
					subnet1.Name = "Subnet0"
					expected := api.EtcdSettings{
						Etcd: api.Etcd{
							EC2Instance: api.EC2Instance{
								Count:        1,
								InstanceType: "t2.medium",
								RootVolume: api.RootVolume{
									Size: 30,
									Type: "gp2",
									IOPS: 0,
								},
								Tenancy: "default",
							},
							DataVolume: api.DataVolume{
								Size:      30,
								Type:      "gp2",
								IOPS:      0,
								Ephemeral: false,
							},
							Cluster: api.EtcdCluster{
								MemberIdentityProvider: "eni",
							},
							Subnets: api.Subnets{
								subnet1,
							},
							UserSuppliedArgs: api.UserSuppliedArgs{
								QuotaBackendBytes: api.DefaultQuotaBackendBytes,
							},
						},
					}
					actual := c.EtcdSettings
					if diff := cmp.Diff(actual, expected); diff != "" {
						t.Errorf("EtcdSettings didn't match: %s", diff)
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
					subnet1 := api.NewPublicSubnet(firstAz, "10.0.0.0/24")
					subnet1.Name = "Subnet0"
					expected := api.EtcdSettings{
						Etcd: api.Etcd{
							Cluster: api.EtcdCluster{
								MemberIdentityProvider: "eni",
								InternalDomainName:     "internal.example.com",
							},
							EC2Instance: api.EC2Instance{
								Count:        1,
								InstanceType: "t2.medium",
								RootVolume: api.RootVolume{
									Size: 30,
									Type: "gp2",
									IOPS: 0,
								},
								Tenancy: "default",
							},
							DataVolume: api.DataVolume{
								Size:      30,
								Type:      "gp2",
								IOPS:      0,
								Ephemeral: false,
							},
							Subnets: api.Subnets{
								subnet1,
							},
							UserSuppliedArgs: api.UserSuppliedArgs{
								QuotaBackendBytes: api.DefaultQuotaBackendBytes,
							},
						},
					}
					actual := c.EtcdSettings
					if diff := cmp.Diff(actual, expected); diff != "" {
						t.Errorf("EtcdSettings didn't match: %s", diff)
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
					subnet1 := api.NewPublicSubnet(firstAz, "10.0.0.0/24")
					subnet1.Name = "Subnet0"
					expected := api.EtcdSettings{
						Etcd: api.Etcd{
							Cluster: api.EtcdCluster{
								MemberIdentityProvider: "eni",
								InternalDomainName:     "internal.example.com",
							},
							EC2Instance: api.EC2Instance{
								Count:        1,
								InstanceType: "t2.medium",
								RootVolume: api.RootVolume{
									Size: 30,
									Type: "gp2",
									IOPS: 0,
								},
								Tenancy: "default",
							},
							DataVolume: api.DataVolume{
								Size:      30,
								Type:      "gp2",
								IOPS:      0,
								Ephemeral: false,
							},
							Nodes: []api.EtcdNode{
								api.EtcdNode{
									FQDN: "etcd1a.internal.example.com",
								},
								api.EtcdNode{
									FQDN: "etcd1b.internal.example.com",
								},
								api.EtcdNode{
									FQDN: "etcd1c.internal.example.com",
								},
							},
							Subnets: api.Subnets{
								subnet1,
							},
							UserSuppliedArgs: api.UserSuppliedArgs{
								QuotaBackendBytes: api.DefaultQuotaBackendBytes,
							},
						},
					}
					actual := c.EtcdSettings
					if diff := cmp.Diff(actual, expected); diff != "" {
						t.Errorf("EtcdSettings didn't match: %s", diff)
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
					subnet1 := api.NewPublicSubnet(firstAz, "10.0.0.0/24")
					subnet1.Name = "Subnet0"
					expected := api.EtcdSettings{
						Etcd: api.Etcd{
							Cluster: api.EtcdCluster{
								MemberIdentityProvider: "eni",
								InternalDomainName:     "internal.example.com",
							},
							EC2Instance: api.EC2Instance{
								Count:        1,
								InstanceType: "t2.medium",
								RootVolume: api.RootVolume{
									Size: 30,
									Type: "gp2",
									IOPS: 0,
								},
								Tenancy: "default",
							},
							DataVolume: api.DataVolume{
								Size:      30,
								Type:      "gp2",
								IOPS:      0,
								Ephemeral: false,
							},
							Nodes: []api.EtcdNode{
								api.EtcdNode{
									Name: "etcd1a",
								},
								api.EtcdNode{
									Name: "etcd1b",
								},
								api.EtcdNode{
									Name: "etcd1c",
								},
							},
							Subnets: api.Subnets{
								subnet1,
							},
							UserSuppliedArgs: api.UserSuppliedArgs{
								QuotaBackendBytes: api.DefaultQuotaBackendBytes,
							},
						},
					}
					actual := c.EtcdSettings
					if diff := cmp.Diff(actual, expected); diff != "" {
						t.Errorf("EtcdSettings didn't match: %s", diff)
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
					subnet1 := api.NewPublicSubnet(firstAz, "10.0.0.0/24")
					subnet1.Name = "Subnet0"
					manageRecordSets := false
					expected := api.EtcdSettings{
						Etcd: api.Etcd{
							Cluster: api.EtcdCluster{
								ManageRecordSets:       &manageRecordSets,
								MemberIdentityProvider: "eni",
								InternalDomainName:     "internal.example.com",
							},
							EC2Instance: api.EC2Instance{
								Count:        1,
								InstanceType: "t2.medium",
								RootVolume: api.RootVolume{
									Size: 30,
									Type: "gp2",
									IOPS: 0,
								},
								Tenancy: "default",
							},
							DataVolume: api.DataVolume{
								Size:      30,
								Type:      "gp2",
								IOPS:      0,
								Ephemeral: false,
							},
							Nodes: []api.EtcdNode{
								api.EtcdNode{
									Name: "etcd1a",
								},
								api.EtcdNode{
									Name: "etcd1b",
								},
								api.EtcdNode{
									Name: "etcd1c",
								},
							},
							Subnets: api.Subnets{
								subnet1,
							},
							UserSuppliedArgs: api.UserSuppliedArgs{
								QuotaBackendBytes: api.DefaultQuotaBackendBytes,
							},
						},
					}
					actual := c.EtcdSettings
					if diff := cmp.Diff(actual, expected); diff != "" {
						t.Errorf("EtcdSettings didn't match: %s", diff)
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
					subnet1 := api.NewPublicSubnet(firstAz, "10.0.0.0/24")
					subnet1.Name = "Subnet0"
					expected := api.EtcdSettings{
						Etcd: api.Etcd{
							Cluster: api.EtcdCluster{
								HostedZone:             api.Identifier{ID: "hostedzone-abcdefg"},
								MemberIdentityProvider: "eni",
								InternalDomainName:     "internal.example.com",
							},
							EC2Instance: api.EC2Instance{
								Count:        1,
								InstanceType: "t2.medium",
								RootVolume: api.RootVolume{
									Size: 30,
									Type: "gp2",
									IOPS: 0,
								},
								Tenancy: "default",
							},
							DataVolume: api.DataVolume{
								Size:      30,
								Type:      "gp2",
								IOPS:      0,
								Ephemeral: false,
							},
							Nodes: []api.EtcdNode{
								api.EtcdNode{
									Name: "etcd1a",
								},
								api.EtcdNode{
									Name: "etcd1b",
								},
								api.EtcdNode{
									Name: "etcd1c",
								},
							},
							Subnets: api.Subnets{
								subnet1,
							},
							UserSuppliedArgs: api.UserSuppliedArgs{
								QuotaBackendBytes: api.DefaultQuotaBackendBytes,
							},
						},
					}
					actual := c.EtcdSettings
					if diff := cmp.Diff(actual, expected); diff != "" {
						t.Errorf("EtcdSettings didn't match: %s", diff)
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
    denyEscalatingExec:
      enabled: true
    alwaysPullImages:
      enabled: true
    priority:
      enabled: true
    mutatingAdmissionWebhook:
      enabled: true
    validatingAdmissionWebhook:
      enabled: true
    persistentVolumeClaimResize:
      enabled: false
  auditLog:
    enabled: true
    logPath: "/var/log/audit.log"
    maxAge: 100
    maxBackup: 10
    maxSize: 5
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
  tlsBootstrap:
    enabled: true
  ephemeralImageStorage:
    enabled: true
  kiamSupport:
    enabled: false
  kube2IamSupport:
    enabled: true
  gpuSupport:
    enabled: true
    version: "375.66"
    installImage: "shelmangroup/coreos-nvidia-driver-installer:latest"
  kubeletOpts: '--image-gc-low-threshold 60 --image-gc-high-threshold 70'
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
  oidc:
    enabled: true
    oidc-issuer-url: "https://accounts.google.com"
    oidc-client-id: "kubernetes"
    oidc-username-claim: "email"
    oidc-groups-claim: "groups"
  nodeDrainer:
    enabled: true
    drainTimeout: 3
cloudWatchLogging:
  enabled: true
amazonSsmAgent:
  enabled: true
worker:
  nodePools:
  - name: pool1
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				asgBasedNodePoolHasWaitSignalEnabled,
				func(c *config.Config, t *testing.T) {
					expected := api.Experimental{
						Admission: api.Admission{
							PodSecurityPolicy: api.PodSecurityPolicy{
								Enabled: true,
							},
							AlwaysPullImages: api.AlwaysPullImages{
								Enabled: true,
							},
							DenyEscalatingExec: api.DenyEscalatingExec{
								Enabled: true,
							},
							Priority: api.Priority{
								Enabled: true,
							},
							MutatingAdmissionWebhook: api.MutatingAdmissionWebhook{
								Enabled: true,
							},
							ValidatingAdmissionWebhook: api.ValidatingAdmissionWebhook{
								Enabled: true,
							},
							PersistentVolumeClaimResize: api.PersistentVolumeClaimResize{
								Enabled: false,
							},
						},
						AuditLog: api.AuditLog{
							Enabled:   true,
							LogPath:   "/var/log/audit.log",
							MaxAge:    100,
							MaxBackup: 10,
							MaxSize:   5,
						},
						Authentication: api.Authentication{
							Webhook: api.Webhook{
								Enabled:  true,
								CacheTTL: "1234s",
								Config:   "e30k",
							},
						},
						AwsEnvironment: api.AwsEnvironment{
							Enabled: true,
							Environment: map[string]string{
								"CFNSTACK": `{ "Ref" : "AWS::StackId" }`,
							},
						},
						AwsNodeLabels: api.AwsNodeLabels{
							Enabled: true,
						},
						ClusterAutoscalerSupport: api.ClusterAutoscalerSupport{
							Enabled: true,
							Options: map[string]string{},
						},
						TLSBootstrap: api.TLSBootstrap{
							Enabled: true,
						},
						EphemeralImageStorage: api.EphemeralImageStorage{
							Enabled:    true,
							Disk:       "xvdb",
							Filesystem: "xfs",
						},
						KIAMSupport: api.KIAMSupport{
							Enabled:         false,
							Image:           api.Image{Repo: "quay.io/uswitch/kiam", Tag: "v2.8", RktPullDocker: false},
							SessionDuration: "15m",
							ServerAddresses: api.KIAMServerAddresses{ServerAddress: "localhost:443", AgentAddress: "kiam-server:443"},
						},
						Kube2IamSupport: api.Kube2IamSupport{
							Enabled: true,
						},
						GpuSupport: api.GpuSupport{
							Enabled:      true,
							Version:      "375.66",
							InstallImage: "shelmangroup/coreos-nvidia-driver-installer:latest",
						},
						KubeletOpts: "--image-gc-low-threshold 60 --image-gc-high-threshold 70",
						LoadBalancer: api.LoadBalancer{
							Enabled:          true,
							Names:            []string{"manuallymanagedlb"},
							SecurityGroupIds: []string{"sg-12345678"},
						},
						TargetGroup: api.TargetGroup{
							Enabled:          true,
							Arns:             []string{"arn:aws:elasticloadbalancing:eu-west-1:xxxxxxxxxxxx:targetgroup/manuallymanagedetg/xxxxxxxxxxxxxxxx"},
							SecurityGroupIds: []string{"sg-12345678"},
						},
						Oidc: api.Oidc{
							Enabled:       true,
							IssuerUrl:     "https://accounts.google.com",
							ClientId:      "kubernetes",
							UsernameClaim: "email",
							GroupsClaim:   "groups",
						},
						NodeDrainer: api.NodeDrainer{
							Enabled:      true,
							DrainTimeout: 3,
						},
					}

					actual := c.Experimental

					if !reflect.DeepEqual(expected, actual) {
						t.Errorf("experimental settings didn't match : expected=%+v actual=%+v", expected, actual)
					}

					p := c.NodePools[0]
					if reflect.DeepEqual(expected, p.Experimental) {
						t.Errorf("experimental settings shouldn't be inherited to a node pool but it did : toplevel=%v nodepool=%v", expected, p.Experimental)
					}

				},
			},
			assertCluster: []ClusterTester{
				hasDefaultCluster,
				func(c *root.Cluster, t *testing.T) {
					cp := c.ControlPlane()
					controllerUserdataS3Part := cp.UserData["Controller"].Parts[api.USERDATA_S3].Asset.Content
					if match, _ := regexp.MatchString(`--feature-gates=.*ExpandPersistentVolumes=false`, controllerUserdataS3Part); !match {
						t.Error("missing controller feature gate: ExpandPersistentVolumes=false")
					}

					if !strings.Contains(controllerUserdataS3Part, `scheduling.k8s.io/v1alpha1=true`) {
						t.Error("missing controller runtime config: scheduling.k8s.io/v1alpha1=true")
					}

					re, _ := regexp.Compile("--enable-admission-plugins=[a-zA-z,]*,Priority")
					if len(re.FindString(controllerUserdataS3Part)) == 0 {
						t.Error("missing controller --enable-admission-plugins config: Priority")
					}

				},
			},
		},
		{
			context: "WithExperimentalFeaturesForWorkerNodePool",
			configYaml: minimalValidConfigYaml + `
addons:
  clusterAutoscaler:
    enabled: true
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
    tlsBootstrap:
      enabled: true # Must be ignored, value is synced with the one from control plane
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
    # Ignored, uses global setting
    nodeDrainer:
      enabled: true
      drainTimeout: 5
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
					expected := api.Experimental{
						AwsEnvironment: api.AwsEnvironment{
							Enabled: true,
							Environment: map[string]string{
								"CFNSTACK": `{ "Ref" : "AWS::StackId" }`,
							},
						},
						AwsNodeLabels: api.AwsNodeLabels{
							Enabled: true,
						},
						ClusterAutoscalerSupport: api.ClusterAutoscalerSupport{
							Enabled: true,
							Options: map[string]string{},
						},
						TLSBootstrap: api.TLSBootstrap{
							Enabled: false,
						},
						EphemeralImageStorage: api.EphemeralImageStorage{
							Enabled:    true,
							Disk:       "xvdb",
							Filesystem: "xfs",
						},
						Kube2IamSupport: api.Kube2IamSupport{
							Enabled: true,
						},
						LoadBalancer: api.LoadBalancer{
							Enabled:          true,
							Names:            []string{"manuallymanagedlb"},
							SecurityGroupIds: []string{"sg-12345678"},
						},
						TargetGroup: api.TargetGroup{
							Enabled:          true,
							Arns:             []string{"arn:aws:elasticloadbalancing:eu-west-1:xxxxxxxxxxxx:targetgroup/manuallymanagedetg/xxxxxxxxxxxxxxxx"},
							SecurityGroupIds: []string{"sg-12345678"},
						},
						NodeDrainer: api.NodeDrainer{
							Enabled:      false,
							DrainTimeout: 0,
						},
					}
					p := c.NodePools[0]
					if reflect.DeepEqual(expected, p.Experimental) {
						t.Errorf("experimental settings for node pool didn't match : expected=%v actual=%v", expected, p.Experimental)
					}

					expectedNodeLabels := api.NodeLabels{
						"kube-aws.coreos.com/cluster-autoscaler-supported": "true",
						"kube-aws.coreos.com/role":                         "worker",
					}
					actualNodeLabels := c.NodePools[0].NodeLabels()
					if !reflect.DeepEqual(expectedNodeLabels, actualNodeLabels) {
						t.Errorf("worker node labels didn't match: expected=%v, actual=%v", expectedNodeLabels, actualNodeLabels)
					}

					expectedTaints := api.Taints{
						{Key: "reservation", Value: "spot", Effect: "NoSchedule"},
					}
					actualTaints := c.NodePools[0].Taints
					if !reflect.DeepEqual(expectedTaints, actualTaints) {
						t.Errorf("worker node taints didn't match: expected=%v, actual=%v", expectedTaints, actualTaints)
					}
				},
			},
		},
		{
			context: "WithExperimentalFeatureKiam",
			configYaml: minimalValidConfigYaml + `
experimental:
  kiamSupport:
    enabled: true
    image:
      repo: quay.io/uswitch/kiam
      tag: v2.6
    sessionDuration: 30m	
    serverAddresses:
      serverAddress: localhost
      agentAddress: kiam-server
worker:
  nodePools:
  - name: pool1
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					expected := api.KIAMSupport{
						Enabled:         true,
						Image:           api.Image{Repo: "quay.io/uswitch/kiam", Tag: "v2.6", RktPullDocker: false},
						SessionDuration: "30m",
						ServerAddresses: api.KIAMServerAddresses{ServerAddress: "localhost", AgentAddress: "kiam-server"},
					}

					actual := c.Experimental

					if !reflect.DeepEqual(expected, actual.KIAMSupport) {
						t.Errorf("experimental settings didn't match : expected=%+v actual=%+v", expected, actual)
					}

					p := c.NodePools[0]
					if reflect.DeepEqual(expected, p.Experimental.KIAMSupport) {
						t.Errorf("experimental settings shouldn't be inherited to a node pool but it did : toplevel=%v nodepool=%v", expected, p.Experimental)
					}
				},
			},
		},
		{
			context: "WithExperimentalFeatureKiamForWorkerNodePool",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    kiamSupport:
      enabled: true
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					expected := api.Experimental{
						KIAMSupport: api.KIAMSupport{
							Enabled:         true,
							Image:           api.Image{Repo: "quay.io/uswitch/kiam", Tag: "v2.8", RktPullDocker: false},
							SessionDuration: "15m",
							ServerAddresses: api.KIAMServerAddresses{ServerAddress: "localhost:443", AgentAddress: "kiam-server:443"},
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
  iam:
    role:
      name: myrole1
experimental:
  kube2IamSupport:
    enabled: true
worker:
  nodePools:
  - name: pool1
    iam:
      role:
        name: myrole2
    kube2IamSupport:
      enabled: true
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				asgBasedNodePoolHasWaitSignalEnabled,
				func(c *config.Config, t *testing.T) {
					expectedControllerRoleName := "myrole1"
					expectedWorkerRoleName := "myrole2"

					if expectedControllerRoleName != c.Controller.IAMConfig.Role.Name {
						t.Errorf("controller's iam.role.name didn't match : expected=%v actual=%v", expectedControllerRoleName, c.Controller.IAMConfig.Role.Name)
					}

					if !c.Experimental.Kube2IamSupport.Enabled {
						t.Errorf("controller's experimental.kube2IamSupport should be enabled but was not: %+v", c.Experimental)
					}

					p := c.NodePools[0]
					if expectedWorkerRoleName != p.IAMConfig.Role.Name {
						t.Errorf("worker node pool's iam.role.name didn't match : expected=%v actual=%v", expectedWorkerRoleName, p.IAMConfig.Role.Name)
					}

					if !p.Kube2IamSupport.Enabled {
						t.Errorf("worker node pool's kube2IamSupport should be enabled but was not: %+v", p.Experimental)
					}
				},
			},
		},
		{
			context:    "WithControllerIAMDefaultManageExternally",
			configYaml: minimalValidConfigYaml,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					expectedValue := false

					if c.Controller.IAMConfig.Role.ManageExternally != expectedValue {
						t.Errorf("controller's iam.role.manageExternally didn't match : expected=%v actual=%v", expectedValue, c.Controller.IAMConfig.Role.ManageExternally)
					}
				},
			},
		},
		{
			context: "WithControllerIAMEnabledManageExternally",
			configYaml: minimalValidConfigYaml + `
controller:
  iam:
   role:
     name: myrole1
     manageExternally: true
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					expectedManageExternally := true
					expectedRoleName := "myrole1"

					if expectedRoleName != c.Controller.IAMConfig.Role.Name {
						t.Errorf("controller's iam.role.name didn't match : expected=%v actual=%v", expectedRoleName, c.Controller.IAMConfig.Role.Name)
					}

					if expectedManageExternally != c.Controller.IAMConfig.Role.ManageExternally {
						t.Errorf("controller's iam.role.manageExternally didn't matchg : expected=%v actual=%v", expectedManageExternally, c.Controller.IAMConfig.Role.ManageExternally)
					}
				},
			},
		},
		{
			context: "WithControllerIAMEnabledStrictName",
			configYaml: minimalValidConfigYaml + `
controller:
  iam:
   role:
     name: myrole1
     strictName: true
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					expectedRoleName := "myrole1"
					if expectedRoleName != c.Controller.IAMConfig.Role.Name {
						t.Errorf("controller's iam.role.name didn't match : expected=%v actual=%v", expectedRoleName, c.Controller.IAMConfig.Role.Name)
					}
				},
			},
		},
		{
			context: "WithWaitSignalDisabled",
			configYaml: minimalValidConfigYaml + `
waitSignal:
  enabled: false
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				func(c *config.Config, t *testing.T) {
					if c.WaitSignal.Enabled() {
						t.Errorf("waitSignal should be disabled but was not: %v", c.WaitSignal)
					}
				},
			},
		},
		{
			context: "WithWaitSignalEnabled",
			configYaml: minimalValidConfigYaml + `
waitSignal:
  enabled: true
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				func(c *config.Config, t *testing.T) {
					if !c.WaitSignal.Enabled() {
						t.Errorf("waitSignal should be enabled but was not: %v", c.WaitSignal)
					}
				},
			},
		},
		{
			context: "WithNodePoolWithWaitSignalDisabled",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    waitSignal:
      enabled: false
  - name: pool2
    waitSignal:
      enabled: false
      maxBatchSize: 2
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				func(c *config.Config, t *testing.T) {
					if c.NodePools[0].WaitSignal.Enabled() {
						t.Errorf("waitSignal should be disabled for node pool at index %d but was not", 0)
					}
					if c.NodePools[1].WaitSignal.Enabled() {
						t.Errorf("waitSignal should be disabled for node pool at index %d but was not", 1)
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
					if !c.NodePools[0].WaitSignal.Enabled() {
						t.Errorf("waitSignal should be enabled for node pool at index %d but was not", 0)
					}
					if c.NodePools[0].WaitSignal.MaxBatchSize() != 1 {
						t.Errorf("waitSignal.maxBatchSize should be 1 for node pool at index %d but was %d", 0, c.NodePools[0].WaitSignal.MaxBatchSize())
					}
					if !c.NodePools[1].WaitSignal.Enabled() {
						t.Errorf("waitSignal should be enabled for node pool at index %d but was not", 1)
					}
					if c.NodePools[1].WaitSignal.MaxBatchSize() != 2 {
						t.Errorf("waitSignal.maxBatchSize should be 2 for node pool at index %d but was %d", 1, c.NodePools[1].WaitSignal.MaxBatchSize())
					}
				},
			},
		},
		{
			context: "WithDefaultNodePoolRollingStrategy",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
`,
			assertConfig: []ConfigTester{
				hasDefaultNodePoolRollingStrategy,
			},
		},
		{
			context: "WithSpecificNodePoolRollingStrategy",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    nodePoolRollingStrategy: Sequential`,
			assertConfig: []ConfigTester{
				hasSpecificNodePoolRollingStrategy("Sequential"),
			},
		},
		{
			context: "WithSpecificWorkerRollingStrategy",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePoolRollingStrategy: Sequential
  nodePools:
  - name: pool1`,
			assertConfig: []ConfigTester{
				hasSpecificNodePoolRollingStrategy("Sequential"),
			},
		},
		{
			context: "WithWorkerAndNodePoolStrategy",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePoolRollingStrategy: Sequential
  nodePools:
  - name: pool1
  - name: pool2
    nodePoolRollingStrategy: Parallel
`,
			assertConfig: []ConfigTester{
				hasWorkerAndNodePoolStrategy("Sequential", "Parallel"),
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
					if c.NodePools[0].Count != 1 {
						t.Errorf("default worker count should be 1 but was: %d", c.NodePools[0].Count)
					}
					if c.NodePools[1].Count != 2 {
						t.Errorf("worker count should be set to 2 but was: %d", c.NodePools[1].Count)
					}
					if c.NodePools[2].Count != 0 {
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
			context: "WithMultiAPIEndpoints",
			configYaml: kubeAwsSettings.mainClusterYamlWithoutAPIEndpoint() + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d

subnets:
- name: privateSubnet1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.1.0/24"
  private: true
- name: privateSubnet2
  availabilityZone: us-west-1b
  instanceCIDR: "10.0.2.0/24"
  private: true
- name: publicSubnet1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.3.0/24"
- name: publicSubnet2
  availabilityZone: us-west-1b
  instanceCIDR: "10.0.4.0/24"

worker:
  # cant be possibly "unversioned" one w/ existing elb because doing so would result in a worker kubelet has chances to
  # connect to multiple masters from different clusters!
  apiEndpointName: versionedPrivate
  # btw apiEndpointName can be defaulted to a private/public managed(hence unstable/possibly versioned but not stable/unversioned)
  # elb/round-robin if and only if there is only one. However we dont do the complex defaulting like that for now.

adminAPIEndpointName: versionedPublic

apiEndpoints:
- name: unversionedPublic
  dnsName: api.example.com
  loadBalancer:
    id: elb-internet-facing
    ##you cant configure existing elb like below
    #private: true
    #subnets:
    #- name: privateSubnet1
    ##hostedZone must be omitted when elb id is specified.
    ##in other words, it your responsibility to create an alias record for the elb
    #hostedZone:
    #  id: hostedzone-private
- name: unversionedPrivate
  dnsName: api.internal.example.com
  loadBalancer:
    id: elb-internal
- name: versionedPublic
  dnsName: v1api.example.com
  loadBalancer:
    subnets:
    - name: publicSubnet1
    hostedZone:
      id: hostedzone-public
- name: versionedPrivate
  dnsName: v1api.internal.example.com
  loadBalancer:
    private: true
    subnets:
    - name: privateSubnet1
    hostedZone:
      id: hostedzone-private
- name: versionedPublicAlt
  dnsName: v1apialt.example.com
  loadBalancer:
    # "private: false" implies all the private subnets defined in the top-level "subnets"
    #subnets:
    #- name: publicSubnet1
    #- name: publicSubnet2
    hostedZone:
      id: hostedzone-public
- name: versionedPrivateAlt
  dnsName: v1apialt.internal.example.com
  loadBalancer:
    private: true
    # "private: true" implies all the private subnets defined in the top-level "subnets"
    #subnets:
    #- name: privateSubnet1
    #- name: privateSubnet2
    hostedZone:
      id: hostedzone-private
- name: addedToCertCommonNames
  dnsName: api-alt.example.com
  loadBalancer:
    managed: false
- name: elbOnly
  dnsName: registerme.example.com
  loadBalancer:
    recordSetManaged: false
`,
			assertCluster: []ClusterTester{
				func(rootCluster *root.Cluster, t *testing.T) {
					c := rootCluster.ControlPlane().Config

					private1 := api.NewPrivateSubnet("us-west-1a", "10.0.1.0/24")
					private1.Name = "privateSubnet1"

					private2 := api.NewPrivateSubnet("us-west-1b", "10.0.2.0/24")
					private2.Name = "privateSubnet2"

					public1 := api.NewPublicSubnet("us-west-1a", "10.0.3.0/24")
					public1.Name = "publicSubnet1"

					public2 := api.NewPublicSubnet("us-west-1b", "10.0.4.0/24")
					public2.Name = "publicSubnet2"
					//private1 := api.NewPrivateSubnetFromFn("us-west-1a", `{"Fn::ImportValue":{"Fn::Sub":"${NetworkStackName}-PrivateSubnet1"}}`)
					//private1.Name = "privateSubnet1"
					//
					//private2 := api.NewPrivateSubnetFromFn("us-west-1b", `{"Fn::ImportValue":{"Fn::Sub":"${NetworkStackName}-PrivateSubnet2"}}`)
					//private2.Name = "privateSubnet2"
					//
					//public1 := api.NewPublicSubnetFromFn("us-west-1a", `{"Fn::ImportValue":{"Fn::Sub":"${NetworkStackName}-PublicSubnet1"}}`)
					//public1.Name = "publicSubnet1"
					//
					//public2 := api.NewPublicSubnetFromFn("us-west-1b", `{"Fn::ImportValue":{"Fn::Sub":"${NetworkStackName}-PublicSubnet2"}}`)
					//public2.Name = "publicSubnet2"

					subnets := api.Subnets{
						private1,
						private2,
						public1,
						public2,
					}
					if !reflect.DeepEqual(c.AllSubnets(), subnets) {
						t.Errorf("Managed subnets didn't match: expected=%+v actual=%+v", subnets, c.AllSubnets())
					}

					publicSubnets := api.Subnets{
						public1,
						public2,
					}

					privateSubnets := api.Subnets{
						private1,
						private2,
					}

					unversionedPublic := c.APIEndpoints["unversionedPublic"]
					unversionedPrivate := c.APIEndpoints["unversionedPrivate"]
					versionedPublic := c.APIEndpoints["versionedPublic"]
					versionedPrivate := c.APIEndpoints["versionedPrivate"]
					versionedPublicAlt := c.APIEndpoints["versionedPublicAlt"]
					versionedPrivateAlt := c.APIEndpoints["versionedPrivateAlt"]
					addedToCertCommonNames := c.APIEndpoints["addedToCertCommonNames"]
					elbOnly := c.APIEndpoints["elbOnly"]

					if len(unversionedPublic.LoadBalancer.Subnets) != 0 {
						t.Errorf("unversionedPublic: subnets shuold be empty but was not: actual=%+v", unversionedPublic.LoadBalancer.Subnets)
					}
					if !unversionedPublic.LoadBalancer.Enabled() {
						t.Errorf("unversionedPublic: it should be enabled as the lb to which controller nodes are added, but it was not: loadBalancer=%+v", unversionedPublic.LoadBalancer)
					}

					if len(unversionedPrivate.LoadBalancer.Subnets) != 0 {
						t.Errorf("unversionedPrivate: subnets shuold be empty but was not: actual=%+v", unversionedPrivate.LoadBalancer.Subnets)
					}
					if !unversionedPrivate.LoadBalancer.Enabled() {
						t.Errorf("unversionedPrivate: it should be enabled as the lb to which controller nodes are added, but it was not: loadBalancer=%+v", unversionedPrivate.LoadBalancer)
					}

					if diff := cmp.Diff(versionedPublic.LoadBalancer.Subnets, api.Subnets{public1}); diff != "" {
						t.Errorf("versionedPublic: subnets didn't match: %s", diff)
					}
					if !versionedPublic.LoadBalancer.Enabled() {
						t.Errorf("versionedPublic: it should be enabled as the lb to which controller nodes are added, but it was not: loadBalancer=%+v", versionedPublic.LoadBalancer)
					}

					if diff := cmp.Diff(versionedPrivate.LoadBalancer.Subnets, api.Subnets{private1}); diff != "" {
						t.Errorf("versionedPrivate: subnets didn't match: %s", diff)
					}
					if !versionedPrivate.LoadBalancer.Enabled() {
						t.Errorf("versionedPrivate: it should be enabled as the lb to which controller nodes are added, but it was not: loadBalancer=%+v", versionedPrivate.LoadBalancer)
					}

					if diff := cmp.Diff(versionedPublicAlt.LoadBalancer.Subnets, publicSubnets); diff != "" {
						t.Errorf("versionedPublicAlt: subnets didn't match: %s", diff)
					}
					if !versionedPublicAlt.LoadBalancer.Enabled() {
						t.Errorf("versionedPublicAlt: it should be enabled as the lb to which controller nodes are added, but it was not: loadBalancer=%+v", versionedPublicAlt.LoadBalancer)
					}

					if diff := cmp.Diff(versionedPrivateAlt.LoadBalancer.Subnets, privateSubnets); diff != "" {
						t.Errorf("versionedPrivateAlt: subnets didn't match: %s", diff)
					}
					if !versionedPrivateAlt.LoadBalancer.Enabled() {
						t.Errorf("versionedPrivateAlt: it should be enabled as the lb to which controller nodes are added, but it was not: loadBalancer=%+v", versionedPrivateAlt.LoadBalancer)
					}

					if len(addedToCertCommonNames.LoadBalancer.Subnets) != 0 {
						t.Errorf("addedToCertCommonNames: subnets should be empty but was not: actual=%+v", addedToCertCommonNames.LoadBalancer.Subnets)
					}
					if addedToCertCommonNames.LoadBalancer.Enabled() {
						t.Errorf("addedToCertCommonNames: it should not be enabled as the lb to which controller nodes are added, but it was: loadBalancer=%+v", addedToCertCommonNames.LoadBalancer)
					}

					if diff := cmp.Diff(elbOnly.LoadBalancer.Subnets, publicSubnets); diff != "" {
						t.Errorf("elbOnly: subnets didn't match: %s", diff)
					}
					if !elbOnly.LoadBalancer.Enabled() {
						t.Errorf("elbOnly: it should be enabled but it was not: loadBalancer=%+v", elbOnly.LoadBalancer)
					}
					if elbOnly.LoadBalancer.ManageELBRecordSet() {
						t.Errorf("elbOnly: record set should not be managed but it was: loadBalancer=%+v", elbOnly.LoadBalancer)
					}

					if diff := cmp.Diff(c.ExternalDNSNames(), []string{"api-alt.example.com", "api.example.com", "api.internal.example.com", "registerme.example.com", "v1api.example.com", "v1api.internal.example.com", "v1apialt.example.com", "v1apialt.internal.example.com"}); diff != "" {
						t.Errorf("unexpected external DNS names: %s", diff)
					}

					if !reflect.DeepEqual(c.APIEndpoints.ManagedELBLogicalNames(), []string{"APIEndpointElbOnlyELB", "APIEndpointVersionedPrivateAltELB", "APIEndpointVersionedPrivateELB", "APIEndpointVersionedPublicAltELB", "APIEndpointVersionedPublicELB"}) {
						t.Errorf("unexpected managed ELB logical names: %s", strings.Join(c.APIEndpoints.ManagedELBLogicalNames(), ", "))
					}
				},
			},
		},
		{
			context: "WithNetworkTopologyExplicitSubnets",
			configYaml: mainClusterYaml + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d
# routeTableId must be omitted
# See https://github.com/kubernetes-incubator/kube-aws/pull/284#issuecomment-275962332
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
					private1 := api.NewPrivateSubnet("us-west-1a", "10.0.1.0/24")
					private1.Name = "private1"

					private2 := api.NewPrivateSubnet("us-west-1b", "10.0.2.0/24")
					private2.Name = "private2"

					public1 := api.NewPublicSubnet("us-west-1a", "10.0.3.0/24")
					public1.Name = "public1"

					public2 := api.NewPublicSubnet("us-west-1b", "10.0.4.0/24")
					public2.Name = "public2"

					subnets := api.Subnets{
						private1,
						private2,
						public1,
						public2,
					}
					if !reflect.DeepEqual(c.AllSubnets(), subnets) {
						t.Errorf("Managed subnets didn't match: expected=%v actual=%v", subnets, c.AllSubnets())
					}

					publicSubnets := api.Subnets{
						public1,
						public2,
					}
					importedPublicSubnets := api.Subnets{
						api.NewPublicSubnetFromFn("us-west-1a", `{"Fn::ImportValue":{"Fn::Sub":"${NetworkStackName}-Public1"}}`),
						api.NewPublicSubnetFromFn("us-west-1b", `{"Fn::ImportValue":{"Fn::Sub":"${NetworkStackName}-Public2"}}`),
					}

					p := c.NodePools[0]
					if !reflect.DeepEqual(p.Subnets, importedPublicSubnets) {
						t.Errorf("Worker subnets didn't match: expected=%v actual=%v", importedPublicSubnets, p.Subnets)
					}

					privateSubnets := api.Subnets{
						private1,
						private2,
					}
					if !reflect.DeepEqual(c.Controller.Subnets, privateSubnets) {
						t.Errorf("Controller subnets didn't match: expected=%v actual=%v", privateSubnets, c.Controller.Subnets)
					}
					if !reflect.DeepEqual(c.Controller.LoadBalancer.Subnets, publicSubnets) {
						t.Errorf("Controller loadbalancer subnets didn't match: expected=%v actual=%v", publicSubnets, c.Controller.LoadBalancer.Subnets)
					}
					if diff := cmp.Diff(c.Etcd.Subnets, privateSubnets); diff != "" {
						t.Errorf("Etcd subnets didn't match: %s", diff)
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
			configYaml: mainClusterYaml + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d
# routeTableId must be omitted
# See https://github.com/kubernetes-incubator/kube-aws/pull/284#issuecomment-275962332
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
					private1 := api.NewPrivateSubnet("us-west-1a", "10.0.1.0/24")
					private1.Name = "private1"

					private2 := api.NewPrivateSubnet("us-west-1b", "10.0.2.0/24")
					private2.Name = "private2"

					public1 := api.NewPublicSubnet("us-west-1a", "10.0.3.0/24")
					public1.Name = "public1"

					public2 := api.NewPublicSubnet("us-west-1b", "10.0.4.0/24")
					public2.Name = "public2"

					subnets := api.Subnets{
						private1,
						private2,
						public1,
						public2,
					}
					if !reflect.DeepEqual(c.AllSubnets(), subnets) {
						t.Errorf("Managed subnets didn't match: expected=%v actual=%v", subnets, c.AllSubnets())
					}

					publicSubnets := api.Subnets{
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
			configYaml: mainClusterYaml + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d
# routeTableId must be omitted
# See https://github.com/kubernetes-incubator/kube-aws/pull/284#issuecomment-275962332
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
					private1 := api.NewPrivateSubnet("us-west-1a", "10.0.1.0/24")
					private1.Name = "private1"

					private2 := api.NewPrivateSubnet("us-west-1b", "10.0.2.0/24")
					private2.Name = "private2"

					public1 := api.NewPublicSubnet("us-west-1a", "10.0.3.0/24")
					public1.Name = "public1"

					public2 := api.NewPublicSubnet("us-west-1b", "10.0.4.0/24")
					public2.Name = "public2"

					subnets := api.Subnets{
						private1,
						private2,
						public1,
						public2,
					}
					if !reflect.DeepEqual(c.AllSubnets(), subnets) {
						t.Errorf("Managed subnets didn't match: expected=%v actual=%v", subnets, c.AllSubnets())
					}

					importedPublicSubnets := api.Subnets{
						api.NewPublicSubnetFromFn("us-west-1a", `{"Fn::ImportValue":{"Fn::Sub":"${NetworkStackName}-Public1"}}`),
						api.NewPublicSubnetFromFn("us-west-1b", `{"Fn::ImportValue":{"Fn::Sub":"${NetworkStackName}-Public2"}}`),
					}
					p := c.NodePools[0]
					if !reflect.DeepEqual(p.Subnets, importedPublicSubnets) {
						t.Errorf("Worker subnets didn't match: expected=%v actual=%v", importedPublicSubnets, p.Subnets)
					}

					privateSubnets := api.Subnets{
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
			configYaml: mainClusterYaml + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d
# routeTableId must be omitted
# See https://github.com/kubernetes-incubator/kube-aws/pull/284#issuecomment-275962332
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
					private1 := api.NewPrivateSubnet("us-west-1a", "10.0.1.0/24")
					private1.Name = "private1"

					private2 := api.NewPrivateSubnet("us-west-1b", "10.0.2.0/24")
					private2.Name = "private2"

					public1 := api.NewPublicSubnet("us-west-1a", "10.0.3.0/24")
					public1.Name = "public1"

					public2 := api.NewPublicSubnet("us-west-1b", "10.0.4.0/24")
					public2.Name = "public2"

					subnets := api.Subnets{
						private1,
						private2,
						public1,
						public2,
					}
					publicSubnets := api.Subnets{
						public1,
						public2,
					}
					privateSubnets := api.Subnets{
						private1,
						private2,
					}
					importedPublicSubnets := api.Subnets{
						api.NewPublicSubnetFromFn("us-west-1a", `{"Fn::ImportValue":{"Fn::Sub":"${NetworkStackName}-Public1"}}`),
						api.NewPublicSubnetFromFn("us-west-1b", `{"Fn::ImportValue":{"Fn::Sub":"${NetworkStackName}-Public2"}}`),
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
			context: "WithNetworkTopologyExistingVaryingSubnets",
			configYaml: mainClusterYaml + `
vpc:
  id: vpc-1a2b3c4d
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
					private1 := api.NewExistingPrivateSubnet("us-west-1a", "subnet-1")
					private1.Name = "private1"

					private2 := api.NewImportedPrivateSubnet("us-west-1b", "mycluster-private-subnet-1")
					private2.Name = "private2"

					public1 := api.NewExistingPublicSubnet("us-west-1a", "subnet-2")
					public1.Name = "public1"

					public2 := api.NewImportedPublicSubnet("us-west-1b", "mycluster-public-subnet-1")
					public2.Name = "public2"

					subnets := api.Subnets{
						private1,
						private2,
						public1,
						public2,
					}
					publicSubnets := api.Subnets{
						public1,
						public2,
					}
					privateSubnets := api.Subnets{
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
			context: "WithNetworkTopologyAllExistingPrivateSubnets",
			configYaml: kubeAwsSettings.mainClusterYamlWithoutAPIEndpoint() + fmt.Sprintf(`
vpc:
  id: vpc-1a2b3c4d
subnets:
- name: private1
  availabilityZone: us-west-1a
  id: subnet-1
  private: true
- name: private2
  availabilityZone: us-west-1b
  idFromStackOutput: mycluster-private-subnet-1
  private: true
controller:
  subnets:
  - name: private1
  - name: private2
etcd:
  subnets:
  - name: private1
  - name: private2
worker:
  nodePools:
  - name: pool1
    subnets:
    - name: private1
    - name: private2
apiEndpoints:
- name: public
  dnsName: "%s"
  loadBalancer:
    hostedZone:
      id: hostedzone-xxxx
    private: true
`, kubeAwsSettings.externalDNSName),
			assertConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				hasNoNGWsOrEIPsOrRoutes,
			},
		},
		{
			context: "WithNetworkTopologyAllExistingPublicSubnets",
			configYaml: mainClusterYaml + `
vpc:
  id: vpc-1a2b3c4d
subnets:
- name: public1
  availabilityZone: us-west-1a
  id: subnet-2
- name: public2
  availabilityZone: us-west-1b
  idFromStackOutput: mycluster-public-subnet-1
etcd:
  subnets:
  - name: public1
  - name: public2
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
			},
		},
		{
			context: "WithNetworkTopologyExistingNATGateways",
			configYaml: mainClusterYaml + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d
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
					private1 := api.NewPrivateSubnetWithPreconfiguredNATGateway("us-west-1a", "10.0.1.0/24", "ngw-11111111")
					private1.Name = "private1"

					private2 := api.NewPrivateSubnetWithPreconfiguredNATGateway("us-west-1b", "10.0.2.0/24", "ngw-22222222")
					private2.Name = "private2"

					public1 := api.NewPublicSubnet("us-west-1a", "10.0.3.0/24")
					public1.Name = "public1"

					public2 := api.NewPublicSubnet("us-west-1b", "10.0.4.0/24")
					public2.Name = "public2"

					subnets := api.Subnets{
						private1,
						private2,
						public1,
						public2,
					}
					publicSubnets := api.Subnets{
						public1,
						public2,
					}
					privateSubnets := api.Subnets{
						private1,
						private2,
					}
					importedPublicSubnets := api.Subnets{
						api.NewPublicSubnetFromFn("us-west-1a", `{"Fn::ImportValue":{"Fn::Sub":"${NetworkStackName}-Public1"}}`),
						api.NewPublicSubnetFromFn("us-west-1b", `{"Fn::ImportValue":{"Fn::Sub":"${NetworkStackName}-Public2"}}`),
					}

					if diff := cmp.Diff(c.AllSubnets(), subnets); diff != "" {
						t.Errorf("Managed subnets didn't match: %s", diff)
					}
					p := c.NodePools[0]
					if diff := cmp.Diff(p.Subnets, importedPublicSubnets); diff != "" {
						t.Errorf("Worker subnets didn't match: %s", diff)
					}
					if diff := cmp.Diff(c.Controller.Subnets, publicSubnets); diff != "" {
						t.Errorf("Controller subnets didn't match: %s", diff)
					}
					if diff := cmp.Diff(c.Controller.LoadBalancer.Subnets, publicSubnets); diff != "" {
						t.Errorf("Controller loadbalancer subnets didn't match: %s", diff)
					}
					if diff := cmp.Diff(c.Etcd.Subnets, privateSubnets); diff != "" {
						t.Errorf("Etcd subnets didn't match: %s", diff)
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
			configYaml: mainClusterYaml + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d
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
					private1 := api.NewPrivateSubnetWithPreconfiguredNATGatewayEIP("us-west-1a", "10.0.1.0/24", "eipalloc-11111111")
					private1.Name = "private1"

					private2 := api.NewPrivateSubnetWithPreconfiguredNATGatewayEIP("us-west-1b", "10.0.2.0/24", "eipalloc-22222222")
					private2.Name = "private2"

					public1 := api.NewPublicSubnet("us-west-1a", "10.0.3.0/24")
					public1.Name = "public1"

					public2 := api.NewPublicSubnet("us-west-1b", "10.0.4.0/24")
					public2.Name = "public2"

					subnets := api.Subnets{
						private1,
						private2,
						public1,
						public2,
					}
					publicSubnets := api.Subnets{
						public1,
						public2,
					}
					privateSubnets := api.Subnets{
						private1,
						private2,
					}
					importedPublicSubnets := api.Subnets{
						api.NewPublicSubnetFromFn("us-west-1a", `{"Fn::ImportValue":{"Fn::Sub":"${NetworkStackName}-Public1"}}`),
						api.NewPublicSubnetFromFn("us-west-1b", `{"Fn::ImportValue":{"Fn::Sub":"${NetworkStackName}-Public2"}}`),
					}

					if diff := cmp.Diff(c.AllSubnets(), subnets); diff != "" {
						t.Errorf("Managed subnets didn't match: %s", diff)
					}
					p := c.NodePools[0]
					if diff := cmp.Diff(p.Subnets, importedPublicSubnets); diff != "" {
						t.Errorf("Worker subnets didn't match: %s", diff)
					}
					if diff := cmp.Diff(c.Controller.Subnets, publicSubnets); diff != "" {
						t.Errorf("Controller subnets didn't match: %s", diff)
					}
					if diff := cmp.Diff(c.Controller.LoadBalancer.Subnets, publicSubnets); diff != "" {
						t.Errorf("Controller loadbalancer subnets didn't match: %s", diff)
					}
					if diff := cmp.Diff(c.Etcd.Subnets, privateSubnets); diff != "" {
						t.Errorf("Etcd subnets didn't match: %s", diff)
					}
				},
			},
		},
		{
			context: "WithNetworkTopologyVaryingPublicSubnets",
			configYaml: mainClusterYaml + `
vpc:
  id: vpc-1a2b3c4d
#required only for the managed subnet "public1"
# "public2" is assumed to have an existing route table and an igw already associated to it
internetGateway:
  id: igw-1a2b3c4d
subnets:
- name: public1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.1.0/24"
- name: public2
  availabilityZone: us-west-1b
  id: subnet-2
controller:
  loadBalancer:
    private: false
etcd:
  subnets:
  - name: public1
  - name: public2
worker:
  nodePools:
  - name: pool1
    subnets:
    - name: public1
    - name: public2
`,
			assertConfig: []ConfigTester{},
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
			context: "WithSpotFleetEnabledWithCustomIamRole",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    spotFleet:
      targetCapacity: 10
      iamFleetRoleArn: custom-iam-role
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
        rootVolume:
          size: 100
`,
			assertConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				spotFleetBasedNodePoolHasWaitSignalDisabled,
				func(c *config.Config, t *testing.T) {
					expected := []api.LaunchSpecification{
						{
							WeightedCapacity: 1,
							InstanceType:     "c4.large",
							SpotPrice:        "",
							// RootVolumeSize was not specified in the configYaml but should default to workerRootVolumeSize * weightedCapacity
							// RootVolumeType was not specified in the configYaml but should default to "gp2"
							RootVolume: api.NewGp2RootVolume(40),
						},
						{
							WeightedCapacity: 2,
							InstanceType:     "c4.xlarge",
							SpotPrice:        "",
							RootVolume:       api.NewGp2RootVolume(100),
						},
					}
					p := c.NodePools[0]
					actual := p.WorkerNodePool.SpotFleet.LaunchSpecifications
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
					expected := []api.LaunchSpecification{
						{
							WeightedCapacity: 1,
							InstanceType:     "m4.large",
							SpotPrice:        "",
							// RootVolumeType was not specified in the configYaml but should default to gp2:
							RootVolume: api.NewGp2RootVolume(40),
						},
						{
							WeightedCapacity: 2,
							InstanceType:     "m4.xlarge",
							SpotPrice:        "",
							RootVolume:       api.NewGp2RootVolume(80),
						},
					}
					p := c.NodePools[0]
					actual := p.WorkerNodePool.SpotFleet.LaunchSpecifications
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
        rootVolume:
          iops: 500
`,
			assertConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				spotFleetBasedNodePoolHasWaitSignalDisabled,
				func(c *config.Config, t *testing.T) {
					expected := []api.LaunchSpecification{
						{
							WeightedCapacity: 1,
							InstanceType:     "c4.large",
							SpotPrice:        "",
							// RootVolumeSize was not specified in the configYaml but should default to workerRootVolumeSize * weightedCapacity
							// RootVolumeIOPS was not specified in the configYaml but should default to workerRootVolumeIOPS * weightedCapacity
							// RootVolumeType was not specified in the configYaml but should default to "io1"
							RootVolume: api.NewIo1RootVolume(40, 100),
						},
						{
							WeightedCapacity: 2,
							InstanceType:     "c4.xlarge",
							SpotPrice:        "",
							// RootVolumeType was not specified in the configYaml but should default to:
							RootVolume: api.NewIo1RootVolume(80, 500),
						},
					}
					p := c.NodePools[0]
					actual := p.WorkerNodePool.SpotFleet.LaunchSpecifications
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
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				hasDefaultExperimentalFeatures,
				func(c *config.Config, t *testing.T) {
					vpcId := "vpc-1a2b3c4d"
					if c.VPC.ID != vpcId {
						t.Errorf("vpc id didn't match: expected=%v actual=%v", vpcId, c.VPC.ID)
					}
					igwId := "igw-1a2b3c4d"
					if c.InternetGateway.ID != igwId {
						t.Errorf("internet gateway id didn't match: expected=%v actual=%v", igwId, c.InternetGateway.ID)
					}
				},
			},
		},
		{
			context: "WithLegacyVpcAndIGWIdSpecified",
			configYaml: minimalValidConfigYaml + `
vpcId: vpc-1a2b3c4d
internetGatewayId: igw-1a2b3c4d
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				hasDefaultExperimentalFeatures,
				func(c *config.Config, t *testing.T) {
					vpcId := "vpc-1a2b3c4d"
					if c.VPC.ID != vpcId {
						t.Errorf("vpc id didn't match: expected=%v actual=%v", vpcId, c.VPC.ID)
					}
					igwId := "igw-1a2b3c4d"
					if c.InternetGateway.ID != igwId {
						t.Errorf("internet gateway id didn't match: expected=%v actual=%v", igwId, c.InternetGateway.ID)
					}
				},
			},
		},
		{
			context: "WithVpcIdAndRouteTableIdSpecified",
			configYaml: mainClusterYaml + `
vpc:
  id: vpc-1a2b3c4d
subnets:
- name: Subnet0
  availabilityZone: ` + firstAz + `
  instanceCIDR: "10.0.0.0/24"
  routeTable:
    id: rtb-1a2b3c4d
`,
			assertConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				func(c *config.Config, t *testing.T) {
					subnet1 := api.NewPublicSubnetWithPreconfiguredRouteTable(firstAz, "10.0.0.0/24", "rtb-1a2b3c4d")
					subnet1.Name = "Subnet0"
					subnets := api.Subnets{
						subnet1,
					}
					expected := api.EtcdSettings{
						Etcd: api.Etcd{
							EC2Instance: api.EC2Instance{
								Count:        1,
								InstanceType: "t2.medium",
								RootVolume: api.RootVolume{
									Size: 30,
									Type: "gp2",
									IOPS: 0,
								},
								Tenancy: "default",
							},
							DataVolume: api.DataVolume{
								Size:      30,
								Type:      "gp2",
								IOPS:      0,
								Ephemeral: false,
							},
							Subnets: subnets,
							UserSuppliedArgs: api.UserSuppliedArgs{
								QuotaBackendBytes: api.DefaultQuotaBackendBytes,
							},
						},
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
    iam:
      role:
        name: "myManagedRole"
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				hasDefaultExperimentalFeatures,
				func(c *config.Config, t *testing.T) {
					if c.NodePools[0].IAMConfig.Role.Name != "myManagedRole" {
						t.Errorf("iam.role.name: expected=myManagedRole actual=%s", c.NodePools[0].IAMConfig.Role.Name)
					}
				},
			},
		},
		{
			context: "WithWorkerManagedPolicies",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    iam:
      role:
        managedPolicies:
         - arn: "arn:aws:iam::aws:policy/AdministratorAccess"
         - arn: "arn:aws:iam::000000000000:policy/myManagedPolicy"
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				hasDefaultExperimentalFeatures,
				func(c *config.Config, t *testing.T) {
					if len(c.NodePools[0].IAMConfig.Role.ManagedPolicies) < 2 {
						t.Errorf("iam.role.managedPolicies: incorrect number of policies expected=2 actual=%d", len(c.NodePools[0].IAMConfig.Role.ManagedPolicies))
					}
					if c.NodePools[0].IAMConfig.Role.ManagedPolicies[0].Arn != "arn:aws:iam::aws:policy/AdministratorAccess" {
						t.Errorf("iam.role.managedPolicies: expected=arn:aws:iam::aws:policy/AdministratorAccess actual=%s", c.NodePools[0].IAMConfig.Role.ManagedPolicies[0].Arn)
					}
					if c.NodePools[0].IAMConfig.Role.ManagedPolicies[1].Arn != "arn:aws:iam::000000000000:policy/myManagedPolicy" {
						t.Errorf("iam.role.managedPolicies: expected=arn:aws:iam::000000000000:policy/myManagedPolicy actual=%s", c.NodePools[0].IAMConfig.Role.ManagedPolicies[1].Arn)
					}
				},
			},
		},
		{
			context: "WithWorkerExistingInstanceProfile",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    iam:
      instanceProfile:
        arn: "arn:aws:iam::000000000000:instance-profile/myInstanceProfile"
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				hasDefaultExperimentalFeatures,
				func(c *config.Config, t *testing.T) {
					if c.NodePools[0].IAMConfig.InstanceProfile.Arn != "arn:aws:iam::000000000000:instance-profile/myInstanceProfile" {
						t.Errorf("existingInstanceProfile: expected=arn:aws:iam::000000000000:instance-profile/myInstanceProfile actual=%s", c.NodePools[0].IAMConfig.InstanceProfile.Arn)
					}
				},
			},
		},
		{
			context: "WithWorkerAndControllerExistingInstanceProfile",
			configYaml: minimalValidConfigYaml + `
controller:
  iam:
    instanceProfile:
      arn: "arn:aws:iam::000000000000:instance-profile/myControllerInstanceProfile"
worker:
  nodePools:
  - name: pool1
    iam:
      instanceProfile:
        arn: "arn:aws:iam::000000000000:instance-profile/myInstanceProfile"
`,
			assertConfig: []ConfigTester{
				hasDefaultEtcdSettings,
				hasDefaultExperimentalFeatures,
				func(c *config.Config, t *testing.T) {
					if c.Controller.IAMConfig.InstanceProfile.Arn != "arn:aws:iam::000000000000:instance-profile/myControllerInstanceProfile" {
						t.Errorf("existingInstanceProfile: expected=arn:aws:iam::000000000000:instance-profile/myControllerInstanceProfile actual=%s", c.Controller.IAMConfig.InstanceProfile.Arn)
					}
					if c.NodePools[0].IAMConfig.InstanceProfile.Arn != "arn:aws:iam::000000000000:instance-profile/myInstanceProfile" {
						t.Errorf("existingInstanceProfile: expected=arn:aws:iam::000000000000:instance-profile/myInstanceProfile actual=%s", c.NodePools[0].IAMConfig.InstanceProfile.Arn)
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
						`{"Fn::ImportValue" : {"Fn::Sub" : "${NetworkStackName}-WorkerSecurityGroup"}}`,
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
						`{"Fn::ImportValue" : {"Fn::Sub" : "${NetworkStackName}-WorkerSecurityGroup"}}`,
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
						`{"Fn::ImportValue" : {"Fn::Sub" : "${NetworkStackName}-WorkerSecurityGroup"}}`,
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
controller:
  tenancy: dedicated
etcd:
  tenancy: dedicated
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					if c.Etcd.Tenancy != "dedicated" {
						t.Errorf("Etcd.Tenancy didn't match: expected=dedicated actual=%s", c.Etcd.Tenancy)
					}
					if c.WorkerTenancy != "dedicated" {
						t.Errorf("WorkerTenancy didn't match: expected=dedicated actual=%s", c.WorkerTenancy)
					}
					if c.Controller.Tenancy != "dedicated" {
						t.Errorf("Controller.Tenancy didn't match: expected=dedicated actual=%s", c.Controller.Tenancy)
					}
				},
			},
		},
		{
			context: "WithControllerNodeLabels",
			configYaml: minimalValidConfigYaml + `
controller:
  nodeLabels:
    kube-aws.coreos.com/role: controller
`,
			assertConfig: []ConfigTester{
				hasDefaultExperimentalFeatures,
				func(c *config.Config, t *testing.T) {
					expected := api.NodeLabels{"kube-aws.coreos.com/role": "controller"}
					actual := c.NodeLabels()
					if !reflect.DeepEqual(expected, actual) {
						t.Errorf("unexpected controller node labels: expected=%v, actual=%v", expected, actual)
					}
				},
			},
		},
		{
			context: "WithSSHAccessAllowedSourceCIDRsSpecified",
			configYaml: minimalValidConfigYaml + `
sshAccessAllowedSourceCIDRs:
- 1.2.3.255/32
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					l := len(c.SSHAccessAllowedSourceCIDRs)
					if l != 1 {
						t.Errorf("unexpected size of sshAccessAllowedSouceCIDRs: %d", l)
						t.FailNow()
					}
					actual := c.SSHAccessAllowedSourceCIDRs[0].String()
					expected := "1.2.3.255/32"
					if actual != expected {
						t.Errorf("unexpected cidr in sshAccessAllowedSourecCIDRs[0]. expected = %s, actual = %s", expected, actual)
					}
				},
			},
		},
		{
			context:    "WithSSHAccessAllowedSourceCIDRsOmitted",
			configYaml: minimalValidConfigYaml,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					l := len(c.SSHAccessAllowedSourceCIDRs)
					if l != 1 {
						t.Errorf("unexpected size of sshAccessAllowedSouceCIDRs: %d", l)
						t.FailNow()
					}
					actual := c.SSHAccessAllowedSourceCIDRs[0].String()
					expected := "0.0.0.0/0"
					if actual != expected {
						t.Errorf("unexpected cidr in sshAccessAllowedSourecCIDRs[0]. expected = %s, actual = %s", expected, actual)
					}
				},
			},
		},
		{
			context: "WithSSHAccessAllowedSourceCIDRsEmptied",
			configYaml: minimalValidConfigYaml + `
sshAccessAllowedSourceCIDRs:
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					l := len(c.SSHAccessAllowedSourceCIDRs)
					if l != 0 {
						t.Errorf("unexpected size of sshAccessAllowedSouceCIDRs: %d", l)
						t.FailNow()
					}
				},
			},
		},
		{
			context: "WithWorkerWithoutGPUSettings",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					enabled := c.NodePools[0].Gpu.Nvidia.Enabled
					if enabled {
						t.Errorf("unexpected enabled of gpu.nvidia: %v.  its default value should be false", enabled)
						t.FailNow()
					}
				},
			},
		},
		{
			context: "WithGPUEnabledWorker",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    instanceType: p2.xlarge
    gpu:
      nvidia:
        enabled: true
        version: "123.45"
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					enabled := c.NodePools[0].Gpu.Nvidia.Enabled
					version := c.NodePools[0].Gpu.Nvidia.Version
					if !enabled {
						t.Errorf("unexpected enabled value of gpu.nvidia: %v.", enabled)
						t.FailNow()
					}
					if version != "123.45" {
						t.Errorf("unexpected version value of gpu.nvidia: %v.", version)
						t.FailNow()
					}
				},
			},
		},
		{
			context: "WithGPUDisabledWorker",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    gpu:
      nvidia:
        enabled: false
        version: "123.45"
`,
			assertConfig: []ConfigTester{
				func(c *config.Config, t *testing.T) {
					enabled := c.NodePools[0].Gpu.Nvidia.Enabled
					version := c.NodePools[0].Gpu.Nvidia.Version
					if enabled {
						t.Errorf("unexpected enabled value of gpu.nvidia: %v.", enabled)
						t.FailNow()
					}
					if version != "123.45" {
						t.Errorf("unexpected version value of gpu.nvidia: %v.", version)
						t.FailNow()
					}
				},
			},
		},
	}

	for _, validCase := range validCases {
		t.Run(validCase.context, func(t *testing.T) {
			configBytes := validCase.configYaml
			// TODO Allow including plugins in test data?
			plugins := []*api.Plugin{}
			providedConfig, err := config.ConfigFromBytes([]byte(configBytes), plugins)
			if err != nil {
				t.Errorf("failed to parse config %s: %+v", configBytes, err)
				t.FailNow()
			}

			t.Run("AssertConfig", func(t *testing.T) {
				for _, assertion := range validCase.assertConfig {
					assertion(providedConfig, t)
				}
			})

			helper.WithDummyCredentials(func(dummyAssetsDir string) {
				var stackTemplateOptions = root.NewOptions(false, false)
				stackTemplateOptions.AssetsDir = dummyAssetsDir
				stackTemplateOptions.ControllerTmplFile = "../../builtin/files/userdata/cloud-config-controller"
				stackTemplateOptions.WorkerTmplFile = "../../builtin/files/userdata/cloud-config-worker"
				stackTemplateOptions.EtcdTmplFile = "../../builtin/files/userdata/cloud-config-etcd"
				stackTemplateOptions.RootStackTemplateTmplFile = "../../builtin/files/stack-templates/root.json.tmpl"
				stackTemplateOptions.NodePoolStackTemplateTmplFile = "../../builtin/files/stack-templates/node-pool.json.tmpl"
				stackTemplateOptions.ControlPlaneStackTemplateTmplFile = "../../builtin/files/stack-templates/control-plane.json.tmpl"
				stackTemplateOptions.NetworkStackTemplateTmplFile = "../../builtin/files/stack-templates/network.json.tmpl"
				stackTemplateOptions.EtcdStackTemplateTmplFile = "../../builtin/files/stack-templates/etcd.json.tmpl"

				cl, err := root.CompileClusterFromConfig(providedConfig, stackTemplateOptions, false)
				if err != nil {
					t.Errorf("failed to create cluster driver : %v", err)
					t.FailNow()
				}

				cl.Context = &model.Context{
					ProvidedEncryptService:  helper.DummyEncryptService{},
					ProvidedCFInterrogator:  helper.DummyCFInterrogator{},
					ProvidedEC2Interrogator: helper.DummyEC2Interrogator{},
					StackTemplateGetter:     helper.DummyStackTemplateGetter{},
				}

				_, err = cl.EnsureAllAssetsGenerated()
				if err != nil {
					t.Errorf("%v", err)
					t.FailNow()
				}

				t.Run("AssertCluster", func(t *testing.T) {
					for _, assertion := range validCase.assertCluster {
						assertion(cl, t)
					}
				})

				t.Run("ValidateTemplates", func(t *testing.T) {
					if err := cl.ValidateTemplates(); err != nil {
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

						report, err := cl.ValidateStack()

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
			context: "WithAPIEndpointLBAPIAccessAllowedSourceCIDRsEmptied",
			configYaml: configYamlWithoutExernalDNSName + `
apiEndpoints:
- name: default
  dnsName: k8s.example.com
  loadBalancer:
    apiAccessAllowedSourceCIDRs:
    hostedZone:
      id: a1b2c4
`,
			expectedErrorMessage: `invalid cluster: invalid apiEndpoint "default" at index 0: invalid loadBalancer: either apiAccessAllowedSourceCIDRs or securityGroupIds must be present. Try not to explicitly empty apiAccessAllowedSourceCIDRs or set one or more securityGroupIDs`,
		},
		{
			context: "WithAutoscalingEnabledButClusterAutoscalerIsDefault",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    autoscaling:
      clusterAutoscaler:
        enabled: true
`,
			expectedErrorMessage: "Autoscaling with cluster-autoscaler can't be enabled for node pools because " +
				"you didn't enabled the cluster-autoscaler addon. Enable it by turning on `addons.clusterAutoscaler.enabled`",
		},
		{
			context: "WithAutoscalingEnabledButClusterAutoscalerIsNot",
			configYaml: minimalValidConfigYaml + `
addons:
  clusterAutoscaler:
    enabled: false
worker:
  nodePools:
  - name: pool1
    autoscaling:
      clusterAutoscaler:
        enabled: true
`,
			expectedErrorMessage: "Autoscaling with cluster-autoscaler can't be enabled for node pools because " +
				"you didn't enabled the cluster-autoscaler addon. Enable it by turning on `addons.clusterAutoscaler.enabled`",
		},
		{
			context: "WithClusterAutoscalerEnabledForControlPlane",
			configYaml: minimalValidConfigYaml + `
controller:
  autoscaling:
    clusterAutoscaler:
      enabled: true
`,
			expectedErrorMessage: "cluster-autoscaler can't be enabled for a control plane because " +
				"allowing so for a group of controller nodes spreading over 2 or more availability zones " +
				"results in unreliability while scaling nodes out.",
		},
		{
			// See https://github.com/kubernetes-incubator/kube-aws/issues/365
			context:              "WithClusterNameContainsDots",
			configYaml:           kubeAwsSettings.withClusterName("my.cluster").minimumValidClusterYaml(),
			expectedErrorMessage: "clusterName(=my.cluster) is malformed. It must consist only of alphanumeric characters, colons, or hyphens",
		},
		{
			context: "WithControllerTaint",
			configYaml: minimalValidConfigYaml + `
controller:
  taints:
  - key: foo
    value: bar
    effect: NoSchedule
`,
			expectedErrorMessage: "`controller.taints` must not be specified because tainting controller nodes breaks the cluster",
		},
		{
			context: "WithElasticFileSystemIdInSpecificNodePoolWithManagedSubnets",
			configYaml: mainClusterYaml + `
subnets:
- name: managed1
  availabilityZone: us-west-1a
  instanceCIDR: 10.0.1.0/24
worker:
  nodePools:
  - name: pool1
    subnets:
    - name: managed1
    elasticFileSystemId: efs-12345
  - name: pool2
`,
			expectedErrorMessage: "invalid node pool at index 0: elasticFileSystemId cannot be specified for a node pool in managed subnet(s), but was: efs-12345",
		},
		{
			context: "WithEtcdAutomatedDisasterRecoveryRequiresAutomatedSnapshot",
			configYaml: minimalValidConfigYaml + `
etcd:
  version: 3
  snapshot:
    automated: false
  disasterRecovery:
    automated: true
`,
			expectedErrorMessage: "`etcd.disasterRecovery.automated` is set to true but `etcd.snapshot.automated` is not - automated disaster recovery requires snapshot to be also automated",
		},
		{
			context: "WithEtcdAutomatedDisasterRecoveryDoesntSupportEtcd2",
			configYaml: minimalValidConfigYaml + `
etcd:
  version: 2
  snapshot:
    automated: true
  disasterRecovery:
    automated: false
`,
			expectedErrorMessage: "`etcd.snapshot.automated` is set to true for enabling automated snapshot. However the feature is available only for etcd version 3",
		},
		{
			context: "WithEtcdAutomatedSnapshotDoesntSupportEtcd2",
			configYaml: minimalValidConfigYaml + `
etcd:
  version: 2
  snapshot:
    automated: false
  disasterRecovery:
    automated: true
`,
			expectedErrorMessage: "`etcd.disasterRecovery.automated` is set to true for enabling automated disaster recovery. However the feature is available only for etcd version 3",
		},
		{
			context: "WithInvalidNodeDrainTimeout",
			configYaml: minimalValidConfigYaml + `
experimental:
  nodeDrainer:
    enabled: true
    drainTimeout: 100
`,
			expectedErrorMessage: "Drain timeout must be an integer between 1 and 60, but was 100",
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
			expectedErrorMessage: "invalid taint effect: UnknownEffect",
		},
		{
			context: "WithLegacyControllerSettingKeys",
			configYaml: minimalValidConfigYaml + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d
routeTableId: rtb-1a2b3c4d
controllerCount: 2
controllerCreateTimeout: PT10M
controllerInstanceType: t2.large
controllerRootVolumeSize: 101
controllerRootVolumeType: io1
controllerRootVolumeIOPS: 102
controllerTenancy: dedicated
`,
			expectedErrorMessage: "unknown keys found: controllerCount, controllerCreateTimeout, controllerInstanceType, controllerRootVolumeIOPS, controllerRootVolumeSize, controllerRootVolumeType, controllerTenancy",
		},
		{
			context: "WithLegacyEtcdSettingKeys",
			configYaml: minimalValidConfigYaml + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d
routeTableId: rtb-1a2b3c4d
etcdCount: 2
etcdTenancy: dedicated
etcdInstanceType: t2.large
etcdRootVolumeSize: 101
etcdRootVolumeType: io1
etcdRootVolumeIOPS: 102
etcdDataVolumeSize: 103
etcdDataVolumeType: io1
etcdDataVolumeIOPS: 104
etcdDataVolumeEncrypted: true
`,
			expectedErrorMessage: "unknown keys found: etcdCount, etcdDataVolumeEncrypted, etcdDataVolumeIOPS, etcdDataVolumeSize, etcdDataVolumeType, etcdInstanceType, etcdRootVolumeIOPS, etcdRootVolumeSize, etcdRootVolumeType, etcdTenancy",
		},
		{
			context: "WithAwsNodeLabelEnabledForTooLongClusterNameAndPoolName",
			configYaml: minimalValidConfigYaml + `
# clusterName + nodePools[].name should be less than or equal to 25 characters or the launch configuration name
# "mykubeawsclustername-mynestedstackname-1N2C4K3LLBEDZ-WorkersLC-BC2S9P3JG2QD" exceeds the limit of 63 characters
# See https://kubernetes.io/docs/user-guide/labels/#syntax-and-character-set
clusterName: my-cluster1 # 11 characters
worker:
  nodePools:
  - name: workernodepool1 # 15 characters
    awsNodeLabels:
      enabled: true
`,
			expectedErrorMessage: "awsNodeLabels can't be enabled for node pool because the total number of characters in clusterName(=\"my-cluster1\") + node pool's name(=\"workernodepool1\") exceeds the limit of 25",
		},
		{
			context: "WithAwsNodeLabelEnabledForTooLongClusterName",
			configYaml: minimalValidConfigYaml + `
# clusterName should be less than or equal to 21 characters or the launch configuration name
# "mykubeawsclustername-mynestedstackname-1N2C4K3LLBEDZ-ControllersLC-BC2S9P3JG2QD" exceeds the limit of 63 characters
# See https://kubernetes.io/docs/user-guide/labels/#syntax-and-character-set
clusterName: mycluster # 9
experimental:
  awsNodeLabels:
     enabled: true
`,
			expectedErrorMessage: "awsNodeLabels can't be enabled for controllers because the total number of characters in clusterName(=\"mycluster\") exceeds the limit of 8",
		},
		{
			context: "WithMultiAPIEndpointsInvalidLB",
			configYaml: kubeAwsSettings.mainClusterYamlWithoutAPIEndpoint() + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d

subnets:
- name: publicSubnet1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.1.0/24"

worker:
  apiEndpointName: unversionedPublic

apiEndpoints:
- name: unversionedPublic
  dnsName: api.example.com
  loadBalancer:
    id: elb-internet-facing
    private: true
    subnets:
    - name: publicSubnet1
    hostedZone:
      id: hostedzone-public
`,
			expectedErrorMessage: "invalid apiEndpoint \"unversionedPublic\" at index 0: invalid loadBalancer: type, private, subnets, hostedZone must be omitted when id is specified to reuse an existing ELB",
		},
		{
			context: "WithMultiAPIEndpointsInvalidWorkerAPIEndpointName",
			configYaml: kubeAwsSettings.mainClusterYamlWithoutAPIEndpoint() + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d

subnets:
- name: publicSubnet1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.1.0/24"

worker:
  # no api endpoint named like that exists!
  apiEndpointName: unknownEndpoint

adminAPIEndpointName: versionedPublic

apiEndpoints:
- name: unversionedPublic
  dnsName: api.example.com
  loadBalancer:
    subnets:
    - name: publicSubnet1
    hostedZone:
      id: hostedzone-public
- name: versionedPublic
  dnsName: apiv1.example.com
  loadBalancer:
    subnets:
    - name: publicSubnet1
    hostedZone:
      id: hostedzone-public
`,
			expectedErrorMessage: "invalid value for worker.apiEndpointName: no API endpoint named \"unknownEndpoint\" found",
		},
		{
			context: "WithMultiAPIEndpointsInvalidWorkerNodePoolAPIEndpointName",
			configYaml: kubeAwsSettings.mainClusterYamlWithoutAPIEndpoint() + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d

subnets:
- name: publicSubnet1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.1.0/24"

worker:
  # this one is ok but...
  apiEndpointName: versionedPublic
  nodePools:
  - name: pool1
    # this one is ng; no api endpoint named this exists!
    apiEndpointName: unknownEndpoint

adminAPIEndpointName: versionedPublic

apiEndpoints:
- name: unversionedPublic
  dnsName: api.example.com
  loadBalancer:
    subnets:
    - name: publicSubnet1
    hostedZone:
      id: hostedzone-public
- name: versionedPublic
  dnsName: apiv1.example.com
  loadBalancer:
    subnets:
    - name: publicSubnet1
    hostedZone:
      id: hostedzone-public
`,
			expectedErrorMessage: "invalid node pool at index 0: failed to find an API endpoint named \"unknownEndpoint\": no API endpoint named \"unknownEndpoint\" defined under the `apiEndpoints[]`",
		},
		{
			context: "WithMultiAPIEndpointsMissingDNSName",
			configYaml: kubeAwsSettings.mainClusterYamlWithoutAPIEndpoint() + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d

subnets:
- name: publicSubnet1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.1.0/24"

apiEndpoints:
- name: unversionedPublic
  dnsName:
  loadBalancer:
    hostedZone:
      id: hostedzone-public
`,
			expectedErrorMessage: "invalid apiEndpoint \"unversionedPublic\" at index 0: dnsName must be set",
		},
		{
			context: "WithMultiAPIEndpointsMissingGlobalAPIEndpointName",
			configYaml: kubeAwsSettings.mainClusterYamlWithoutAPIEndpoint() + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d

subnets:
- name: publicSubnet1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.1.0/24"

worker:
  nodePools:
  - name: pool1
    # this one is ng; no api endpoint named this exists!
    apiEndpointName: unknownEndpoint
  - name: pool1
    # this one is ng; missing apiEndpointName

adminAPIEndpointName: versionedPublic

apiEndpoints:
- name: unversionedPublic
  dnsName: api.example.com
  loadBalancer:
    subnets:
    - name: publicSubnet1
    hostedZone:
      id: hostedzone-public
- name: versionedPublic
  dnsName: apiv1.example.com
  loadBalancer:
    subnets:
    - name: publicSubnet1
    hostedZone:
      id: hostedzone-public
`,
			expectedErrorMessage: "worker.apiEndpointName must not be empty when there're 2 or more API endpoints under the key `apiEndpoints` and one of worker.nodePools[] are missing apiEndpointName",
		},
		{
			context: "WithMultiAPIEndpointsRecordSetImpliedBySubnetsMissingHostedZoneID",
			configYaml: kubeAwsSettings.mainClusterYamlWithoutAPIEndpoint() + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d

subnets:
- name: publicSubnet1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.1.0/24"

worker:
  apiEndpointName: unversionedPublic

apiEndpoints:
- name: unversionedPublic
  dnsName: api.example.com
  loadBalancer:
    # an internet-facing(which is the default) lb in the public subnet is going to be created with a corresponding record set
    # however no hosted zone for the record set is provided!
    subnets:
    - name: publicSubnet1
    # missing hosted zone id here!
`,
			expectedErrorMessage: "invalid apiEndpoint \"unversionedPublic\" at index 0: invalid loadBalancer: missing hostedZone.id",
		},
		{
			context: "WithMultiAPIEndpointsRecordSetImpliedByExplicitPublicMissingHostedZoneID",
			configYaml: kubeAwsSettings.mainClusterYamlWithoutAPIEndpoint() + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d

subnets:
- name: publicSubnet1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.1.0/24"

worker:
  apiEndpointName: unversionedPublic

apiEndpoints:
- name: unversionedPublic
  dnsName: api.example.com
  loadBalancer:
    # an internet-facing lb is going to be created with a corresponding record set
    # however no hosted zone for the record set is provided!
    private: false
    # missing hosted zone id here!
`,
			expectedErrorMessage: "invalid apiEndpoint \"unversionedPublic\" at index 0: invalid loadBalancer: missing hostedZone.id",
		},
		{
			context: "WithMultiAPIEndpointsRecordSetImpliedByExplicitPrivateMissingHostedZoneID",
			configYaml: kubeAwsSettings.mainClusterYamlWithoutAPIEndpoint() + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d

subnets:
- name: publicSubnet1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.1.0/24"
- name: privateSubnet1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.2.0/24"

worker:
  apiEndpointName: unversionedPublic

apiEndpoints:
- name: unversionedPublic
  dnsName: api.example.com
  loadBalancer:
    # an internal lb is going to be created with a corresponding record set
    # however no hosted zone for the record set is provided!
    private: true
    # missing hosted zone id here!
`,
			expectedErrorMessage: "invalid apiEndpoint \"unversionedPublic\" at index 0: invalid loadBalancer: missing hostedZone.id",
		},
		{
			context: "WithNetworkTopologyAllExistingPrivateSubnetsRejectingExistingIGW",
			configYaml: mainClusterYaml + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d
subnets:
- name: private1
  availabilityZone: us-west-1a
  id: subnet-1
  private: true
controller:
  loadBalancer:
    private: true
etcd:
  subnets:
  - name: private1
worker:
  nodePools:
  - name: pool1
    subnets:
    - name: private1
`,
			expectedErrorMessage: `internet gateway id can't be specified when all the subnets are existing private subnets`,
		},
		{
			context: "WithNetworkTopologyAllExistingPublicSubnetsRejectingExistingIGW",
			configYaml: mainClusterYaml + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d
subnets:
- name: public1
  availabilityZone: us-west-1a
  id: subnet-1
controller:
  loadBalancer:
    private: false
etcd:
  subnets:
  - name: public1
worker:
  nodePools:
  - name: pool1
    subnets:
    - name: public1
`,
			expectedErrorMessage: `internet gateway id can't be specified when all the public subnets have existing route tables associated. kube-aws doesn't try to modify an exisinting route table to include a route to the internet gateway`,
		},
		{
			context: "WithNetworkTopologyAllManagedPublicSubnetsWithExistingRouteTableRejectingExistingIGW",
			configYaml: mainClusterYaml + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d
subnets:
- name: public1
  availabilityZone: us-west-1a
  instanceCIDR: 10.0.1.0/24
  routeTable:
    id: subnet-1
controller:
  loadBalancer:
    private: false
etcd:
  subnets:
  - name: public1
worker:
  nodePools:
  - name: pool1
    subnets:
    - name: public1
`,
			expectedErrorMessage: `internet gateway id can't be specified when all the public subnets have existing route tables associated. kube-aws doesn't try to modify an exisinting route table to include a route to the internet gateway`,
		},
		{
			context: "WithNetworkTopologyAllManagedPublicSubnetsMissingExistingIGW",
			configYaml: mainClusterYaml + `
vpc:
  id: vpc-1a2b3c4d
#misses this
#internetGateway:
#  id: igw-1a2b3c4d
subnets:
- name: public1
  availabilityZone: us-west-1a
  instanceCIDR: "10.0.1.0/24"
controller:
  loadBalancer:
    private: false
etcd:
  subnets:
  - name: public1
worker:
  nodePools:
  - name: pool1
    subnets:
    - name: public1
`,
			expectedErrorMessage: `internet gateway id can't be omitted when there're one or more managed public subnets in an existing VPC`,
		},
		{
			context: "WithNetworkTopologyAllPreconfiguredPrivateDeprecatedAndThenRemoved",
			configYaml: mainClusterYaml + `
vpc:
  id: vpc-1a2b3c4d
# This, in combination with mapPublicIPs=false, had been implying that the route table contains a route to a preconfigured NAT gateway
# See https://github.com/kubernetes-incubator/kube-aws/pull/284#issuecomment-276008202
routeTableId: rtb-1a2b3c4d
# This had been implied that all the subnets created by kube-aws should be private
mapPublicIPs: false
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
			expectedErrorMessage: "internet gateway id can't be omitted when there're one or more managed public subnets in an existing VPC",
		},
		{
			context: "WithNetworkTopologyAllPreconfiguredPublicDeprecatedAndThenRemoved",
			configYaml: mainClusterYaml + `
vpc:
  id: vpc-1a2b3c4d
# This, in combination with mapPublicIPs=true, had been implying that the route table contains a route to a preconfigured internet gateway
# See https://github.com/kubernetes-incubator/kube-aws/pull/284#issuecomment-276008202
routeTableId: rtb-1a2b3c4d
# This had been implied that all the subnets created by kube-aws should be public
mapPublicIPs: true
# internetGateway.id should be omitted as we assume that the route table specified by routeTableId already contain a route to one
#internetGateway:
#  id:
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
			expectedErrorMessage: "internet gateway id can't be omitted when there're one or more managed public subnets in an existing VPC",
		},
		{
			context: "WithVpcIdAndVPCCIDRSpecified",
			configYaml: minimalValidConfigYaml + `
vpc:
  id: vpc-1a2b3c4d
internetGateway:
  id: igw-1a2b3c4d
# vpcCIDR (10.1.0.0/16) does not contain instanceCIDR (10.0.1.0/24)
vpcCIDR: "10.1.0.0/16"
`,
		},
		{
			context: "WithRouteTableIdSpecified",
			configYaml: minimalValidConfigYaml + `
# vpc.id must be specified if routeTableId is specified
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
		{
			context: "WithUnknownKeyInRoot",
			configYaml: minimalValidConfigYaml + `
foo: bar
`,
			expectedErrorMessage: "unknown keys found: foo",
		},
		{
			context: "WithUnknownKeyInController",
			configYaml: minimalValidConfigYaml + `
controller:
  foo: 1
`,
			expectedErrorMessage: "unknown keys found in controller: foo",
		},
		{
			context: "WithUnknownKeyInControllerASG",
			configYaml: minimalValidConfigYaml + `
controller:
  autoScalingGroup:
    foo: 1
`,
			expectedErrorMessage: "unknown keys found in controller.autoScalingGroup: foo",
		},
		{
			context: "WithUnknownKeyInEtcd",
			configYaml: minimalValidConfigYaml + `
etcd:
  foo: 1
`,
			expectedErrorMessage: "unknown keys found in etcd: foo",
		},
		{
			context: "WithUnknownKeyInWorkerNodePool",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    clusterAutoscaler:
      enabled: true
`,
			expectedErrorMessage: "unknown keys found in worker.nodePools[0]: clusterAutoscaler",
		},
		{
			context: "WithUnknownKeyInWorkerNodePoolASG",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    autoScalingGroup:
      foo: 1
`,
			expectedErrorMessage: "unknown keys found in worker.nodePools[0].autoScalingGroup: foo",
		},
		{
			context: "WithUnknownKeyInWorkerNodePoolSpotFleet",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    spotFleet:
      bar: 1
`,
			expectedErrorMessage: "unknown keys found in worker.nodePools[0].spotFleet: bar",
		},
		{
			context: "WithUnknownKeyInWorkerNodePoolCA",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    autoscaling:
      clusterAutoscaler:
        baz: 1
`,
			expectedErrorMessage: "unknown keys found in worker.nodePools[0].autoscaling.clusterAutoscaler: baz",
		},
		{
			context: "WithUnknownKeyInAddons",
			configYaml: minimalValidConfigYaml + `
addons:
  blah: 5
`,
			expectedErrorMessage: "unknown keys found in addons: blah",
		},
		{
			context: "WithUnknownKeyInReschedulerAddon",
			configYaml: minimalValidConfigYaml + `
addons:
  rescheduler:
    foo: yeah
`,
			expectedErrorMessage: "unknown keys found in addons.rescheduler: foo",
		},
		{
			context: "WithUnknownKeyInClusterAutoscalerAddon",
			configYaml: minimalValidConfigYaml + `
addons:
  clusterAutoscaler:
    foo: yeah
`,
			expectedErrorMessage: "unknown keys found in addons.clusterAutoscaler: foo",
		},
		{
			context: "WithTooLongControllerIAMRoleName",
			configYaml: kubeAwsSettings.withClusterName("kubeaws-it-main").withRegion("ap-northeast-1").minimumValidClusterYaml() + `
controller:
  iam:
    role:
      name: foobarba-foobarba-foobarba-foobarba-foobarba-foobarba
`,
			expectedErrorMessage: "IAM role name(=kubeaws-it-main-ap-northeast-1-foobarba-foobarba-foobarba-foobarba-foobarba-foobarba) will be 84 characters long. It exceeds the AWS limit of 64 characters: clusterName(=kubeaws-it-main) + region name(=ap-northeast-1) + managed iam role name(=foobarba-foobarba-foobarba-foobarba-foobarba-foobarba) should be less than or equal to 33",
		},
		{
			context: "WithTooLongWorkerIAMRoleName",
			configYaml: kubeAwsSettings.withClusterName("kubeaws-it-main").withRegion("ap-northeast-1").minimumValidClusterYaml() + `
worker:
  nodePools:
  - name: pool1
    iam:
      role:
        name: foobarba-foobarba-foobarba-foobarba-foobarba-foobarbazzz
`,
			expectedErrorMessage: "IAM role name(=kubeaws-it-main-ap-northeast-1-foobarba-foobarba-foobarba-foobarba-foobarba-foobarbazzz) will be 87 characters long. It exceeds the AWS limit of 64 characters: clusterName(=kubeaws-it-main) + region name(=ap-northeast-1) + managed iam role name(=foobarba-foobarba-foobarba-foobarba-foobarba-foobarbazzz) should be less than or equal to 33",
		},
		{
			context: "WithInvalidEtcdInstanceProfileArn",
			configYaml: minimalValidConfigYaml + `
etcd:
  iam:
    instanceProfile:
      arn: "badArn"
`,
			expectedErrorMessage: "invalid etcd settings: invalid instance profile, your instance profile must match (=arn:aws:iam::YOURACCOUNTID:instance-profile/INSTANCEPROFILENAME), provided (badArn)",
		},
		{
			context: "WithInvalidEtcdManagedPolicyArn",
			configYaml: minimalValidConfigYaml + `
etcd:
  iam:
    role:
      managedPolicies:
      - arn: "badArn"
`,
			expectedErrorMessage: "invalid etcd settings: invalid managed policy arn, your managed policy must match this (=arn:aws:iam::(YOURACCOUNTID|aws):policy/POLICYNAME), provided this (badArn)",
		},
		{
			context: "WithInvalidWorkerInstanceProfileArn",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    iam:
      instanceProfile:
        arn: "badArn"
`,
			expectedErrorMessage: "invalid instance profile, your instance profile must match (=arn:aws:iam::YOURACCOUNTID:instance-profile/INSTANCEPROFILENAME), provided (badArn)",
		},
		{
			context: "WithInvalidWorkerManagedPolicyArn",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    iam:
      role:
        managedPolicies:
          - arn: "badArn"
`,
			expectedErrorMessage: "invalid managed policy arn, your managed policy must match this (=arn:aws:iam::(YOURACCOUNTID|aws):policy/POLICYNAME), provided this (badArn)",
		},
		{
			context: "WithGPUEnabledWorkerButEmptyVersion",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    instanceType: p2.xlarge
    gpu:
      nvidia:
        enabled: true
        version: ""
`,
			expectedErrorMessage: `gpu.nvidia.version must not be empty when gpu.nvidia is enabled.`,
		},
		{
			context: "WithGPUDisabledWorkerButIntallationSupportEnabled",
			configYaml: minimalValidConfigYaml + `
worker:
  nodePools:
  - name: pool1
    instanceType: t2.medium
    gpu:
      nvidia:
        enabled: true
        version: ""
`,
			expectedErrorMessage: `instance type t2.medium doesn't support GPU. You can enable Nvidia driver intallation support only when use [p2 p3 g2 g3] instance family.`,
		},
	}

	for _, invalidCase := range parseErrorCases {
		t.Run(invalidCase.context, func(t *testing.T) {
			configBytes := invalidCase.configYaml
			// TODO Allow including plugins in test data?
			plugins := []*api.Plugin{}
			providedConfig, err := config.ConfigFromBytes([]byte(configBytes), plugins)
			if err == nil {
				t.Errorf("expected to fail parsing config %s: %+v: %+v", configBytes, *providedConfig, err)
				t.FailNow()
			}

			errorMsg := fmt.Sprintf("%v", err)
			if !strings.Contains(errorMsg, invalidCase.expectedErrorMessage) {
				t.Errorf(`expected "%s" to be contained in the error message : %s`, invalidCase.expectedErrorMessage, errorMsg)
			}
		})
	}
}
