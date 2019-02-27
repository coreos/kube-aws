package model

import (
	"fmt"
	"net"
	"reflect"
	"strings"
	"testing"

	"encoding/json"

	"github.com/kubernetes-incubator/kube-aws/netutil"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
)

const chinaAPIEndpointMinimalConfigYaml = `keyName: test-key-name
region: cn-north-1
s3URI: s3://mybucket/mydir
availabilityZone: cn-north-1a
clusterName: test-cluster-name
`

const minimalChinaConfigYaml = externalDNSNameConfig + chinaAPIEndpointMinimalConfigYaml

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

var incorrectNetworkingConfigs = []string{
	`
vpcCIDR: 10.4.2.0/23
instanceCIDR: 10.4.3.0/24
podCIDR: 10.4.0.0/16 #podCIDR contains vpcCDIR.
serviceCIDR: 10.5.0.0/16
dnsServiceIP: 10.5.100.101
`,
	`
vpcCIDR: 10.4.2.0/23
instanceCIDR: 10.4.3.0/24
podCIDR: 10.5.0.0/16
serviceCIDR: 10.4.0.0/16 #serviceCIDR contains vpcCDIR.
dnsServiceIP: 10.4.100.101
`, `
vpcCIDR: 10.4.0.0/16
instanceCIDR: 10.5.3.0/24 #instanceCIDR not in vpcCIDR
podCIDR: 10.6.0.0/16
serviceCIDR: 10.5.0.0/16
dnsServiceIP: 10.5.100.101
`, `
vpcCIDR: 10.4.3.0/16
instanceCIDR: 10.4.3.0/24
podCIDR: 172.4.0.0/16
serviceCIDR: 172.5.0.0/16
dnsServiceIP: 172.5.0.1 #dnsServiceIP conflicts with kubernetesServiceIP
`, `
vpcCIDR: 10.4.3.0/16
instanceCIDR: 10.4.3.0/24
podCIDR: 10.4.0.0/16 #vpcCIDR overlaps with podCIDR
serviceCIDR: 172.5.0.0/16
dnsServiceIP: 172.5.100.101

`, `
vpcCIDR: 10.4.3.0/16
instanceCIDR: 10.4.3.0/24
podCIDR: 172.4.0.0/16
serviceCIDR: 172.5.0.0/16
dnsServiceIP: 172.6.100.101 #dnsServiceIP not in service CIDR
`, `

subnets:
- name: Subnet0
  instanceCIDR: "10.0.0.0/24"
  availabilityZone: us-west-1c
  routeTable:
    id: rtb-xxxxxx # routeTable.id specified without vpcId
`,
}

var goodAPIEndpointConfigs = []string{
	`
apiEndpoints:
- name: public
  dnsName: test.staging.core-os.net
  loadBalancer:
    recordSetManaged: false
`, `
apiEndpoints:
- name: public
  dnsName: test.staging.core-os.net
  loadBalancer:
    type: classic
    recordSetManaged: false
    securityGroupIds: []
    apiAccessAllowedSourceCIDRs:
      - 0.0.0.0/0
`, `
apiEndpoints:
- name: public
  dnsName: test.staging.core-os.net
  loadBalancer:
    type: network
    recordSetManaged: false
`, `
apiEndpoints:
- name: public
  dnsName: test.staging.core-os.net
  loadBalancer:
    type: network
    recordSetManaged: false
    securityGroupIds: []
`, `
apiEndpoints:
- name: public
  dnsName: test.staging.core-os.net
  loadBalancer:
    type: network
    recordSetManaged: false
    securityGroupIds: []
    apiAccessAllowedSourceCIDRs: []
`,
}

var incorrectAPIEndpointConfigs = []string{
	`
# hostedZone.id shouldn't be blank when recordSetManaged is true
apiEndpoints:
- name: public
  loadBalancer:
    hostedZone:
      id:
    recordSetManaged: true

`, `
# hostedZone.id shouldn't be blank when recordSetManaged is true(=default)
apiEndpoints:
- name: public
  loadBalancer:
    hostedZone:
      id:
`, `
# recordSetTTL shouldn't be modified when you're going to manage the hostname yourself(=hostedZone.id is nil and recordSetManaged is false)
apiEndpoints:
- name: public
  loadBalancer:
    hostedZone:
      id:
    recordSetManaged: false
    recordSetTTL: 400
`, `
# hostedZoneId shouldn't be modified when recordSetManaged is false
apiEndpoints:
- name: public
  loadBalancer:
    hostedZone:
      id: hostedzone-xxxxxx
    recordSetManaged: false
`, `
# recordSetTTL should be greater than zero
apiEndpoints:
- name: public
  loadBalancer:
    hostedZone:
      id: hostedzone-xxxxxx
    recordSetTTL: 0
`, `
# type is invalid
apiEndpoints:
- name: public
  loadBalancer:
    type: invalid
    hostedZone:
      id: hostedzone-xxxxxx
    recordSetTTL: 0
`, `
# cannot set security groups for a network load balancer
apiEndpoints:
- name: public
  loadBalancer:
    type: network
    hostedZone:
      id: hostedzone-xxxxxx
    securityGroupIds:
      - sg-1234
`, `
# must specify either securityGroupIds or apiAccessAllowedSourceCIDRs for classic ELBs
apiEndpoints:
- name: public
  dnsName: test.staging.core-os.net
  loadBalancer:
    type: classic
    recordSetManaged: false
    securityGroupIds: []
    apiAccessAllowedSourceCIDRs: []
`,
}

var featureGates = `
controller:
  featureGates:
    feature1: "true"
    feature2: "false"
`

func TestFeatureFlags(t *testing.T) {
	var c *api.Cluster
	var err error
	if c, err = ClusterFromBytes([]byte(singleAzConfigYaml + featureGates)); err != nil {
		t.Errorf("Incorrect config for controller feature gates: %s\n%s", err, featureGates)
	}
	if c.ControllerFeatureGates().Enabled() != true {
		t.Errorf("Incorrect config for controller feature gates: %s\n%s", err, featureGates)
	}
	if !(c.ControllerFeatureGates()["feature1"] == "true" &&
		c.ControllerFeatureGates()["feature2"] == "false") {
		t.Errorf("Incorrect config for controller feature gates: %s\n%s", err, featureGates)
	}
}

func TestNetworkValidation(t *testing.T) {
	for _, networkConfig := range goodNetworkingConfigs {
		configBody := singleAzConfigYaml + networkConfig
		if _, err := ClusterFromBytes([]byte(configBody)); err != nil {
			t.Errorf("Correct config tested invalid: %s\n%s", err, networkConfig)
		}
	}

	for _, networkConfig := range incorrectNetworkingConfigs {
		configBody := singleAzConfigYaml + networkConfig
		if _, err := ClusterFromBytes([]byte(configBody)); err == nil {
			t.Errorf("Incorrect config tested valid, expected error:\n%s", networkConfig)
		}
	}
}

func TestAPIEndpointValidation(t *testing.T) {
	for _, networkConfig := range goodAPIEndpointConfigs {
		configBody := apiEndpointMinimalConfigYaml + availabilityZoneConfig + networkConfig
		if _, err := ClusterFromBytes([]byte(configBody)); err != nil {
			t.Errorf("Correct config tested invalid: %s\n%s", err, networkConfig)
		}
	}

	for _, networkConfig := range incorrectAPIEndpointConfigs {
		configBody := apiEndpointMinimalConfigYaml + availabilityZoneConfig + networkConfig
		if _, err := ClusterFromBytes([]byte(configBody)); err == nil {
			t.Errorf("Incorrect config tested valid, expected error:\n%s", networkConfig)
		}
	}
}

func TestMinimalChinaConfig(t *testing.T) {
	c, err := ClusterFromBytes([]byte(minimalChinaConfigYaml))
	if err != nil {
		t.Errorf("Failed to parse config %s: %v", minimalChinaConfigYaml, err)
	}

	if !c.Region.IsChina() {
		t.Error("IsChinaRegion test failed.")
	}

	if c.AssetsEncryptionEnabled() {
		t.Error("Assets encryption must be disabled on China.")
	}

	if c.Region.SupportsNetworkLoadBalancers() {
		t.Error("Network load balancers are still not supported on China.")
	}
}

func TestAPIAccessAllowedSourceCIDRsForControllerSG(t *testing.T) {
	testCases := []struct {
		conf  string
		cidrs []string
	}{
		{
			conf:  externalDNSNameConfig,
			cidrs: []string{},
		},
		{
			conf: `
apiEndpoints:
- name: endpoint-1
  dnsName: test-1.staging.core-os.net
  loadBalancer:
    type: network
    recordSetManaged: false
    apiAccessAllowedSourceCIDRs: []
`,
			cidrs: []string{},
		},
		{
			conf: `
apiEndpoints:
- name: endpoint-1
  dnsName: test-1.staging.core-os.net
  loadBalancer:
    type: network
    recordSetManaged: false
`,
			cidrs: []string{"0.0.0.0/0"},
		},
		{
			conf: `
apiEndpoints:
- name: endpoint-1
  dnsName: test-1.staging.core-os.net
  loadBalancer:
    type: network
    recordSetManaged: false
    apiAccessAllowedSourceCIDRs:
      - 127.0.0.1/32

# Ignores non-network load balancers
- name: endpoint-2
  dnsName: test-1.staging.core-os.net
  loadBalancer:
    recordSetManaged: false
    apiAccessAllowedSourceCIDRs:
      - 127.0.0.2/32

# Ignores non-network load balancers
- name: endpoint-2
  dnsName: test-1.staging.core-os.net
  loadBalancer:
    type: classic
    recordSetManaged: false
    apiAccessAllowedSourceCIDRs:
      - 127.0.0.3/32
`,
			cidrs: []string{"127.0.0.1/32"},
		},
		{
			conf: `
apiEndpoints:
- name: endpoint-1
  dnsName: test-1.staging.core-os.net
  loadBalancer:
    type: network
    recordSetManaged: false
    apiAccessAllowedSourceCIDRs:
      - 127.0.0.1/32
      - 0.0.0.0/0

- name: endpoint-2
  dnsName: test-2.staging.core-os.net
  loadBalancer:
    type: network
    recordSetManaged: false
    apiAccessAllowedSourceCIDRs:
      - 127.0.0.1/32   # Duplicated CIDR
      - 192.168.0.0/24
`,
			cidrs: []string{"0.0.0.0/0", "127.0.0.1/32", "192.168.0.0/24"},
		},
	}

	for _, testCase := range testCases {
		confBody := availabilityZoneConfig + apiEndpointMinimalConfigYaml + testCase.conf
		c, err := ClusterFromBytes([]byte(confBody))
		if err != nil {
			t.Errorf("Unexpected error parsing config: %v\n %s", err, confBody)
			continue
		}

		actualCIDRs := c.APIAccessAllowedSourceCIDRsForControllerSG()
		if !reflect.DeepEqual(actualCIDRs, testCase.cidrs) {
			t.Errorf(
				"CIDRs %s do not match actual list %s in config: %s",
				testCase.cidrs,
				actualCIDRs,
				confBody,
			)
		}
	}
}

func TestKubernetesServiceIPInference(t *testing.T) {

	// We sill assert that after parsing the network configuration,
	// KubernetesServiceIP is the correct pre-determined value
	testConfigs := []struct {
		NetworkConfig       string
		KubernetesServiceIP string
	}{
		{
			NetworkConfig: `
serviceCIDR: 172.5.10.10/22
dnsServiceIP: 172.5.10.10
        `,
			KubernetesServiceIP: "172.5.8.1",
		},
		{
			NetworkConfig: `
serviceCIDR: 10.5.70.10/18
dnsServiceIP: 10.5.64.10
        `,
			KubernetesServiceIP: "10.5.64.1",
		},
		{
			NetworkConfig: `
serviceCIDR: 172.4.155.98/27
dnsServiceIP: 172.4.155.100
        `,
			KubernetesServiceIP: "172.4.155.97",
		},
		{
			NetworkConfig: `
serviceCIDR: 10.6.142.100/28
dnsServiceIP: 10.6.142.100
        `,
			KubernetesServiceIP: "10.6.142.97",
		},
	}

	for _, testConfig := range testConfigs {
		configBody := singleAzConfigYaml + testConfig.NetworkConfig
		cluster, err := ClusterFromBytes([]byte(configBody))
		if err != nil {
			t.Errorf("Unexpected error parsing config: %v\n %s", err, configBody)
			continue
		}

		_, serviceNet, err := net.ParseCIDR(cluster.ServiceCIDR)
		if err != nil {
			t.Errorf("invalid serviceCIDR: %v", err)
			continue
		}

		kubernetesServiceIP := netutil.IncrementIP(serviceNet.IP)
		if kubernetesServiceIP.String() != testConfig.KubernetesServiceIP {
			t.Errorf("KubernetesServiceIP mismatch: got %s, expected %s",
				kubernetesServiceIP,
				testConfig.KubernetesServiceIP)
		}
	}
}

func TestReleaseChannel(t *testing.T) {

	validConfigs := []struct {
		conf    string
		channel string
	}{
		{
			conf: `
releaseChannel: alpha
`,
			channel: "alpha",
		},
		{
			conf: `
releaseChannel: beta
`,
			channel: "beta",
		},
		{
			conf: `
releaseChannel: stable
`,
			channel: "stable",
		},
	}

	invalidConfigs := []string{
		`
releaseChannel: non-existent #this release channel will never exist
`,
	}

	for _, conf := range validConfigs {
		confBody := singleAzConfigYaml + conf.conf
		c, err := ClusterFromBytes([]byte(confBody))
		if err != nil {
			t.Errorf("failed to parse config %s: %v", confBody, err)
			continue
		}
		if c.ReleaseChannel != conf.channel {
			t.Errorf(
				"parsed release channel %s does not match config: %s",
				c.ReleaseChannel,
				confBody,
			)
		}
	}

	for _, conf := range invalidConfigs {
		confBody := singleAzConfigYaml + conf
		_, err := ClusterFromBytes([]byte(confBody))
		if err == nil {
			t.Errorf("expected error parsing invalid config: %s", confBody)
		}
	}
}

func TestAvailabilityZones(t *testing.T) {
	testCases := []struct {
		conf string
		azs  []string
	}{
		{
			conf: singleAzConfigYaml,
			azs:  []string{"us-west-1c"},
		},
		{
			conf: minimalConfigYaml + `
# You can specify multiple subnets to be created in order to achieve H/A
vpcCIDR: 10.4.3.0/16
subnets:
  - availabilityZone: ap-northeast-1a
    instanceCIDR: 10.4.3.0/24
  - availabilityZone: ap-northeast-1c
    instanceCIDR: 10.4.4.0/24
`,
			azs: []string{"ap-northeast-1a", "ap-northeast-1c"},
		},
	}

	for _, conf := range testCases {
		confBody := conf.conf
		c, err := ClusterFromBytes([]byte(confBody))
		if err != nil {
			t.Errorf("failed to parse config %s: %v", confBody, err)
			continue
		}

		actualAzs := c.AvailabilityZones()
		if !reflect.DeepEqual(actualAzs, conf.azs) {
			t.Errorf(
				"availability zones %s do not match actual list %s in config: %s",
				conf.azs,
				actualAzs,
				confBody,
			)
		}
	}
}

func TestMultipleSubnets(t *testing.T) {

	validConfigs := []struct {
		conf    string
		subnets api.Subnets
	}{
		{
			conf: `
# You can specify multiple subnets to be created in order to achieve H/A
vpcCIDR: 10.4.3.0/16
subnets:
  - availabilityZone: ap-northeast-1a
    instanceCIDR: 10.4.3.0/24
  - availabilityZone: ap-northeast-1c
    instanceCIDR: 10.4.4.0/24
`,
			subnets: api.Subnets{
				{
					InstanceCIDR:     "10.4.3.0/24",
					AvailabilityZone: "ap-northeast-1a",
					Name:             "Subnet0",
				},
				{
					InstanceCIDR:     "10.4.4.0/24",
					AvailabilityZone: "ap-northeast-1c",
					Name:             "Subnet1",
				},
			},
		},
		{
			conf: `
# Given AZ/CIDR, missing subnets fall-back to the single subnet with the AZ/CIDR given.
vpcCIDR: 10.4.3.0/16
availabilityZone: ap-northeast-1a
instanceCIDR: 10.4.3.0/24
`,
			subnets: api.Subnets{
				{
					AvailabilityZone: "ap-northeast-1a",
					InstanceCIDR:     "10.4.3.0/24",
					Name:             "Subnet0",
				},
			},
		},
		{
			conf: `
# Given AZ/CIDR, empty subnets fall-back to the single subnet with the AZ/CIDR given.
vpcCIDR: 10.4.3.0/16
availabilityZone: ap-northeast-1a
instanceCIDR: 10.4.3.0/24
subnets: []
`,
			subnets: api.Subnets{
				{
					AvailabilityZone: "ap-northeast-1a",
					InstanceCIDR:     "10.4.3.0/24",
					Name:             "Subnet0",
				},
			},
		},
		{
			conf: `
# Given no AZ/CIDR, empty subnets fall-backs to the single subnet with the default az/cidr.
availabilityZone: "ap-northeast-1a"
subnets: []
`,
			subnets: api.Subnets{
				{
					AvailabilityZone: "ap-northeast-1a",
					InstanceCIDR:     "10.0.0.0/24",
					Name:             "Subnet0",
				},
			},
		},
		{
			conf: `
# Missing subnets field fall-backs to the single subnet with the default az/cidr.
availabilityZone: "ap-northeast-1a"
`,
			subnets: api.Subnets{
				{
					AvailabilityZone: "ap-northeast-1a",
					InstanceCIDR:     "10.0.0.0/24",
					Name:             "Subnet0",
				},
			},
		},
	}

	invalidConfigs := []string{
		`
# You can't specify both the top-level availability zone and subnets
# (It doesn't make sense. Which configuration did you want, single or multi AZ one?)
availabilityZone: "ap-northeast-1a"
subnets:
  - availabilityZone: "ap-northeast-1b"
    instanceCIDR: "10.0.0.0/24"
`,
		`
# You can't specify both the top-level instanceCIDR and subnets
# (It doesn't make sense. Which configuration did you want, single or multi AZ one?)
instanceCIDR: "10.0.0.0/24"
subnets:
- availabilityZone: "ap-northeast-1b"
  instanceCIDR: "10.0.1.0/24"
`,
		`
subnets:
# Missing AZ like this
# - availabilityZone: "ap-northeast-1a"
- instanceCIDR: 10.0.0.0/24
`,
		`
subnets:
# Missing AZ like this
# - availabilityZone: "ap-northeast-1a"
- instanceCIDR: 10.0.0.0/24
`,
		`
subnets:
# Overlapping subnets
- availabilityZone: "ap-northeast-1a"
  instanceCIDR: 10.0.5.0/24
- availabilityZone: "ap-northeast-1b"
  instanceCIDR: 10.0.5.0/24
`,
	}

	for _, conf := range validConfigs {
		confBody := minimalConfigYaml + conf.conf
		c, err := ClusterFromBytes([]byte(confBody))
		if err != nil {
			t.Errorf("failed to parse config %s: %v", confBody, err)
			continue
		}
		if !reflect.DeepEqual(c.Subnets, conf.subnets) {
			t.Errorf(
				"parsed subnets %+v does not match expected subnets %+v in config: %s",
				c.Subnets,
				conf.subnets,
				confBody,
			)
		}
	}

	for _, conf := range invalidConfigs {
		confBody := minimalConfigYaml + conf
		_, err := ClusterFromBytes([]byte(confBody))
		if err == nil {
			t.Errorf("expected error parsing invalid config:\n%s", confBody)
		}
	}

}

func TestControllerVolumeType(t *testing.T) {

	validConfigs := []struct {
		conf       string
		volumeType string
		iops       int
	}{
		{
			conf:       ``,
			volumeType: "gp2",
			iops:       0,
		},
		{
			conf: `
controller:
  rootVolume:
    type: gp2
`,
			volumeType: "gp2",
			iops:       0,
		},
		{
			conf: `
controller:
  rootVolume:
    type: standard
`,
			volumeType: "standard",
			iops:       0,
		},
		{
			conf: `
controller:
  rootVolume:
    type: io1
    iops: 100
`,
			volumeType: "io1",
			iops:       100,
		},
		{
			conf: `
controller:
  rootVolume:
    type: io1
    iops: 20000
`,
			volumeType: "io1",
			iops:       20000,
		},
	}

	invalidConfigs := []string{
		`
# There's no volume type 'default'
controller:
  rootVolume:
    type: default
`,
		`
# IOPS must be zero for volume types != 'io1'
controller:
  rootVolume:
    type: standard
    iops: 100
`,
		`
# IOPS must be zero for volume types != 'io1'
controller:
  rootVolume:
    type: gp2
    iops: 20000
`,
		`
# IOPS smaller than the minimum (100)
controller:
  rootVolume:
    type: io1
    iops: 99
`,
		`
# IOPS greater than the maximum (20000)
controller:
  rootVolume:
    type: io1
    iops: 20001
`,
	}

	for _, conf := range validConfigs {
		confBody := singleAzConfigYaml + conf.conf
		c, err := ClusterFromBytes([]byte(confBody))
		if err != nil {
			t.Errorf("failed to parse config %s: %v", confBody, err)
			continue
		}
		if c.Controller.RootVolume.Type != conf.volumeType {
			t.Errorf(
				"parsed root volume type %s does not match root volume %s in config: %s",
				c.Controller.RootVolume.Type,
				conf.volumeType,
				confBody,
			)
		}
	}

	for _, conf := range invalidConfigs {
		confBody := singleAzConfigYaml + conf
		_, err := ClusterFromBytes([]byte(confBody))
		if err == nil {
			t.Errorf("expected error parsing invalid config: %s", confBody)
		}
	}
}

func TestWorkerVolumeType(t *testing.T) {

	validConfigs := []struct {
		conf       string
		volumeType string
		iops       int
	}{
		{
			conf:       ``,
			volumeType: "gp2",
			iops:       0,
		},
		{
			conf: `
workerRootVolumeType: gp2
`,
			volumeType: "gp2",
			iops:       0,
		},
		{
			conf: `
workerRootVolumeType: standard
`,
			volumeType: "standard",
			iops:       0,
		},
		{
			conf: `
workerRootVolumeType: io1
workerRootVolumeIOPS: 100
`,
			volumeType: "io1",
			iops:       100,
		},
		{
			conf: `
workerRootVolumeType: io1
workerRootVolumeIOPS: 20000
`,
			volumeType: "io1",
			iops:       20000,
		},
	}

	invalidConfigs := []string{
		`
# There's no volume type 'default'
workerRootVolumeType: default
`,
		`
# IOPS must be zero for volume types != 'io1'
workerRootVolumeType: standard
workerRootVolumeIOPS: 100
`,
		`
# IOPS must be zero for volume types != 'io1'
workerRootVolumeType: gp2
workerRootVolumeIOPS: 20000
`,
		`
# IOPS smaller than the minimum (100)
workerRootVolumeType: io1
workerRootVolumeIOPS: 99
`,
		`
# IOPS greater than the maximum (20000)
workerRootVolumeType: io1
workerRootVolumeIOPS: 20001
`,
	}

	for _, conf := range validConfigs {
		confBody := singleAzConfigYaml + conf.conf
		c, err := ClusterFromBytes([]byte(confBody))
		if err != nil {
			t.Errorf("failed to parse config %s: %v", confBody, err)
			continue
		}
		if c.WorkerRootVolumeType != conf.volumeType {
			t.Errorf(
				"parsed root volume type %s does not match root volume %s in config: %s",
				c.WorkerRootVolumeType,
				conf.volumeType,
				confBody,
			)
		}
	}

	for _, conf := range invalidConfigs {
		confBody := singleAzConfigYaml + conf
		_, err := ClusterFromBytes([]byte(confBody))
		if err == nil {
			t.Errorf("expected error parsing invalid config: %s", confBody)
		}
	}
}

func TestNodeDrainerConfig(t *testing.T) {

	validConfigs := []struct {
		conf        string
		nodeDrainer api.NodeDrainer
	}{
		{
			conf: `
`,
			nodeDrainer: api.NodeDrainer{
				Enabled:      false,
				DrainTimeout: 5,
				IAMRole:      api.IAMRole{},
			},
		},
		{
			conf: `
experimental:
  nodeDrainer:
    enabled: true
    iamRole:
      arn: arn:aws:iam::0123456789012:role/asg-list-role
`,
			nodeDrainer: api.NodeDrainer{
				Enabled:      true,
				DrainTimeout: 5,
				IAMRole:      api.IAMRole{ARN: api.ARN{Arn: "arn:aws:iam::0123456789012:role/asg-list-role"}},
			},
		},
		{
			conf: `
experimental:
  nodeDrainer:
    enabled: true
    drainTimeout: 3
`,
			nodeDrainer: api.NodeDrainer{
				Enabled:      true,
				DrainTimeout: 3,
			},
		},
	}

	for _, conf := range validConfigs {
		confBody := singleAzConfigYaml + conf.conf
		c, err := ClusterFromBytes([]byte(confBody))
		if err != nil {
			t.Errorf("failed to parse config %s: %v", confBody, err)
			continue
		}
		if !reflect.DeepEqual(c.Experimental.NodeDrainer, conf.nodeDrainer) {
			t.Errorf(
				"parsed node drainer settings %+v does not match config: %s",
				c.Experimental.NodeDrainer,
				confBody,
			)
		}
	}

}

func TestEncryptionAtRestConfig(t *testing.T) {

	validConfigs := []struct {
		conf             string
		encryptionAtRest api.EncryptionAtRest
	}{
		{
			conf: `
`,
			encryptionAtRest: api.EncryptionAtRest{
				Enabled: false,
			},
		},
		{
			conf: `
kubernetes:
  encryptionAtRest:
    enabled: false
`,
			encryptionAtRest: api.EncryptionAtRest{
				Enabled: false,
			},
		},
		{
			conf: `
kubernetes:
  encryptionAtRest:
    enabled: true
`,
			encryptionAtRest: api.EncryptionAtRest{
				Enabled: true,
			},
		},
		{
			conf: `
# Settings for an experimental feature must be under the "experimental" field. Ignored.
encryptionAtRest:
  enabled: true
`,
			encryptionAtRest: api.EncryptionAtRest{
				Enabled: false,
			},
		},
	}

	for _, conf := range validConfigs {
		confBody := singleAzConfigYaml + conf.conf
		c, err := ClusterFromBytes([]byte(confBody))
		if err != nil {
			y, err2 := json.MarshalIndent(c, "", "  ")
			if err2 != nil {
				t.Errorf("%v", err2)
				t.FailNow()
			}
			t.Logf("%s", string(y))
			t.Errorf("failed to parse config: %v:\n%s", err, confBody)
			continue
		}
		if !reflect.DeepEqual(c.Kubernetes.EncryptionAtRest, conf.encryptionAtRest) {
			t.Errorf(
				"parsed encryption at rest settings %+v does not match config: %s",
				c.Kubernetes.EncryptionAtRest,
				confBody,
			)
		}
	}
}

func TestRotateCerts(t *testing.T) {

	validConfigs := []struct {
		conf        string
		rotateCerts api.RotateCerts
	}{
		{
			conf: `
`,
			rotateCerts: api.RotateCerts{
				Enabled: false,
			},
		},
		{
			conf: `
kubelet:
  rotateCerts:
    enabled: false
`,
			rotateCerts: api.RotateCerts{
				Enabled: false,
			},
		},
		{
			conf: `
kubelet:
  rotateCerts:
    enabled: true
`,
			rotateCerts: api.RotateCerts{
				Enabled: true,
			},
		},
		{
			conf: `
rotateCerts:
  enabled: true
`,
			rotateCerts: api.RotateCerts{
				Enabled: false,
			},
		},
	}

	for _, conf := range validConfigs {
		confBody := singleAzConfigYaml + conf.conf
		c, err := ClusterFromBytes([]byte(confBody))
		if err != nil {
			t.Errorf("failed to parse config %s: %v", confBody, err)
			continue
		}
		if !reflect.DeepEqual(c.Kubelet.RotateCerts, conf.rotateCerts) {
			t.Errorf(
				"parsed Rotate Certificates settings %+v does not match config: %s",
				c.Kubelet.RotateCerts,
				confBody,
			)
		}
	}
}

func TestKubeletReserved(t *testing.T) {

	validConfigs := []struct {
		conf           string
		kubeReserved   string
		systemReserved string
	}{
		{
			conf: `
`,
			systemReserved: "",
			kubeReserved:   "",
		},
		{
			conf: `
kubelet:
  kubeReserved: "cpu=100m,memory=100Mi,ephemeral-storage=1Gi"
  systemReserved: "cpu=200m,memory=200Mi,ephemeral-storage=2Gi"
`,
			kubeReserved:   "cpu=100m,memory=100Mi,ephemeral-storage=1Gi",
			systemReserved: "cpu=200m,memory=200Mi,ephemeral-storage=2Gi",
		},
		{
			conf: `
kubeReserved: "cpu=100m,memory=100Mi,ephemeral-storage=1Gi"
systemReserved: "cpu=200m,memory=200Mi,ephemeral-storage=2Gi"
`,
			kubeReserved:   "",
			systemReserved: "",
		},
	}

	for _, conf := range validConfigs {
		confBody := singleAzConfigYaml + conf.conf
		c, err := ClusterFromBytes([]byte(confBody))
		if err != nil {
			t.Errorf("failed to parse config %s: %v", confBody, err)
			continue
		}
		if !reflect.DeepEqual(c.Kubelet.KubeReservedResources, conf.kubeReserved) || !reflect.DeepEqual(c.Kubelet.SystemReservedResources, conf.systemReserved) {
			t.Errorf(
				"parsed KubeReservedResources (%+v) and/or SystemReservedResources (%+v) settings does not match config: %s",
				c.Kubelet.KubeReservedResources,
				c.Kubelet.SystemReservedResources,
				confBody,
			)
		}
	}
}

func TestKubeDns(t *testing.T) {

	validConfigs := []struct {
		conf    string
		kubeDns api.KubeDns
	}{
		{
			conf: `
`,
			kubeDns: api.KubeDns{
				Provider:            "kube-dns",
				NodeLocalResolver:   false,
				DeployToControllers: false,
				Autoscaler: api.KubeDnsAutoscaler{
					CoresPerReplica: 256,
					NodesPerReplica: 16,
					Min:             2,
				},
			},
		},
		{
			conf: `
kubeDns:
  nodeLocalResolver: false
  deployToControllers: false
`,
			kubeDns: api.KubeDns{
				Provider:            "kube-dns",
				NodeLocalResolver:   false,
				DeployToControllers: false,
				Autoscaler: api.KubeDnsAutoscaler{
					CoresPerReplica: 256,
					NodesPerReplica: 16,
					Min:             2,
				},
			},
		},
		{
			conf: `
kubeDns:
  nodeLocalResolver: true
  deployToControllers: true
  autoscaler:
    coresPerReplica: 5
    nodesPerReplica: 10
    min: 15
`,
			kubeDns: api.KubeDns{
				Provider:            "kube-dns",
				NodeLocalResolver:   true,
				DeployToControllers: true,
				Autoscaler: api.KubeDnsAutoscaler{
					CoresPerReplica: 5,
					NodesPerReplica: 10,
					Min:             15,
				},
			},
		},
		{
			conf: `
kubeDns:
  provider: coredns
`,
			kubeDns: api.KubeDns{
				Provider:            "coredns",
				NodeLocalResolver:   false,
				DeployToControllers: false,
				Autoscaler: api.KubeDnsAutoscaler{
					CoresPerReplica: 256,
					NodesPerReplica: 16,
					Min:             2,
				},
			},
		},
	}

	for _, conf := range validConfigs {
		confBody := singleAzConfigYaml + conf.conf
		c, err := ClusterFromBytes([]byte(confBody))
		if err != nil {
			t.Errorf("failed to parse config %s: %v", confBody, err)
			continue
		}
		if !reflect.DeepEqual(c.KubeDns, conf.kubeDns) {
			t.Errorf(
				"parsed kubeDns settings %+v does not match config: %s",
				c.KubeDns,
				confBody,
			)
		}
	}
}

func TestTLSBootstrapConfig(t *testing.T) {

	validConfigs := []struct {
		conf         string
		tlsBootstrap api.TLSBootstrap
	}{
		{
			conf: `
`,
			tlsBootstrap: api.TLSBootstrap{
				Enabled: false,
			},
		},
		{
			conf: `
experimental:
  tlsBootstrap:
    enabled: false
`,
			tlsBootstrap: api.TLSBootstrap{
				Enabled: false,
			},
		},
		{
			conf: `
experimental:
  tlsBootstrap:
    enabled: true
`,
			tlsBootstrap: api.TLSBootstrap{
				Enabled: true,
			},
		},
		{
			conf: `
# Settings for an experimental feature must be under the "experimental" field. Ignored.
tlsBootstrap:
  enabled: true
`,
			tlsBootstrap: api.TLSBootstrap{
				Enabled: false,
			},
		},
	}

	for _, conf := range validConfigs {
		confBody := singleAzConfigYaml + conf.conf
		c, err := ClusterFromBytes([]byte(confBody))
		if err != nil {
			t.Errorf("failed to parse config %s: %v", confBody, err)
			continue
		}
		if !reflect.DeepEqual(c.Experimental.TLSBootstrap, conf.tlsBootstrap) {
			t.Errorf(
				"parsed TLS bootstrap settings %+v does not match config: %s",
				c.Experimental.TLSBootstrap,
				confBody,
			)
		}
	}
}

func TestNodeAuthorizerConfig(t *testing.T) {
	validConfigs := []struct {
		conf           string
		nodeAuthorizer api.NodeAuthorizer
	}{
		{
			conf: `
`,
			nodeAuthorizer: api.NodeAuthorizer{
				Enabled: false,
			},
		},
		{
			conf: `
experimental:
  nodeAuthorizer:
    enabled: false
`,
			nodeAuthorizer: api.NodeAuthorizer{
				Enabled: false,
			},
		},
		{
			conf: `
experimental:
  nodeAuthorizer:
    enabled: true
  tlsBootstrap:
    enabled: true
`,
			nodeAuthorizer: api.NodeAuthorizer{
				Enabled: true,
			},
		},
	}

	invalidConfigs := []string{
		`
# TLS bootstrap must be enabled as well
experimental:
  nodeAuthorizer:
    enabled: true
`,
		`
# TLS bootstrap must be enabled as well
experimental:
  nodeAuthorizer:
    enabled: true
  tlsBootstrap:
    enabled: false
`}

	for _, conf := range validConfigs {
		confBody := singleAzConfigYaml + conf.conf
		c, err := ClusterFromBytes([]byte(confBody))
		if err != nil {
			t.Errorf("failed to parse config %s: %v", confBody, err)
			continue
		}
		if !reflect.DeepEqual(c.Experimental.NodeAuthorizer, conf.nodeAuthorizer) {
			t.Errorf(
				"parsed node authorizer settings %+v does not match config: %s",
				c.Experimental.NodeAuthorizer,
				confBody,
			)
		}
	}

	for _, conf := range invalidConfigs {
		confBody := singleAzConfigYaml + conf
		_, err := ClusterFromBytes([]byte(confBody))
		if err == nil {
			t.Errorf("expected error parsing invalid config: %s", confBody)
		}
	}
}

func TestRktConfig(t *testing.T) {
	validChannels := []string{
		"alpha",
		"beta",
		"stable",
	}

	conf := func(channel string) string {
		return fmt.Sprintf(`containerRuntime: rkt
releaseChannel: %s
`, channel)
	}

	for _, channel := range validChannels {
		confBody := singleAzConfigYaml + conf(channel)
		_, err := ClusterFromBytes([]byte(confBody))
		if err != nil {
			t.Errorf("failed to parse config %s: %v", confBody, err)
		}
	}
}

func TestValidateExistingVPC(t *testing.T) {
	validCases := []struct {
		vpc     string
		subnets []string
	}{
		{"10.0.0.0/16", []string{"10.0.3.0/24", "10.0.4.0/24"}},
	}

	invalidCases := []struct {
		vpc     string
		subnets []string
	}{
		// both subnets conflicts
		{"10.0.0.0/16", []string{"10.0.1.0/24", "10.0.2.0/24"}},
		// 10.0.1.0/24 conflicts
		{"10.0.0.0/16", []string{"10.0.1.0/24", "10.0.3.0/24"}},
		// 10.0.2.0/24 conflicts
		{"10.0.0.0/16", []string{"10.0.2.0/24", "10.0.3.0/24"}},
		// vpc cidr doesn't match
		{"10.1.0.0/16", []string{"10.1.1.0/24", "10.1.2.0/24"}},
		// vpc cidr is invalid
		{"1o.1.o.o/16", []string{"10.1.1.0/24", "10.1.2.0/24"}},
		// subnet cidr is invalid
		{"10.1.0.0/16", []string{"1o.1.1.o/24", "10.1.2.0/24"}},
	}

	cluster := api.NewDefaultCluster()

	cluster.VPCCIDR = "10.0.0.0/16"
	cluster.Subnets = api.Subnets{
		api.NewPublicSubnet("ap-northeast-1a", "10.0.1.0/24"),
		api.NewPublicSubnet("ap-northeast-1a", "10.0.2.0/24"),
	}

	for _, testCase := range validCases {
		err := cluster.ValidateExistingVPC(testCase.vpc, testCase.subnets)

		if err != nil {
			t.Errorf("failed to validate existing vpc and subnets: %v", err)
		}
	}

	for _, testCase := range invalidCases {
		err := cluster.ValidateExistingVPC(testCase.vpc, testCase.subnets)

		if err == nil {
			t.Errorf("expected to fail validating existing vpc and subnets: %v", testCase)
		}
	}
}

func TestInvalidKubernetesVersion(t *testing.T) {
	testCases := []string{
		`
kubernetesVersion: v1.x.3
`,
		`
kubernetesVersion: v1.11.3yes
`,
		`
kubernetesVersion: $v1.11.3
`}

	for _, testCase := range testCases {
		confBody := singleAzConfigYaml + testCase
		_, err := ClusterFromBytes([]byte(confBody))
		if err == nil || !strings.Contains(err.Error(), "must be a valid version") {
			t.Errorf("expected kubernetesVersion to be validated: %s\n%s", err, confBody)

		}
	}
}

func TestValidKubernetesVersion(t *testing.T) {
	testCases := []string{
		`
kubernetesVersion: v1.11.3
`,
		`
kubernetesVersion: v1.7.2
`}

	for _, testCase := range testCases {
		confBody := singleAzConfigYaml + testCase
		_, err := ClusterFromBytes([]byte(confBody))
		if err != nil {
			t.Errorf("expected kubernetesVersion to be validated: %s\n%s", err, confBody)
		}
	}
}

func TestApiServerLeaseEndpointReconcilerDisabled(t *testing.T) {
	testCases := []string{
		`
kubernetesVersion: v1.7.16
`,
		`
kubernetesVersion: v1.8.12
`}

	for _, testCase := range testCases {
		confBody := singleAzConfigYaml + testCase
		c, _ := ClusterFromBytes([]byte(confBody))
		if enabled, err := c.ApiServerLeaseEndpointReconciler(); enabled == true || err != nil {
			t.Errorf("API server lease endpoint should not be enabled prior to Kubernetes 1.9: %s\n%s", err, confBody)
		}
	}
}
func TestApiServerLeaseEndpointReconcilerEnabled(t *testing.T) {
	testCases := []string{
		`
kubernetesVersion: v1.11.3
`,
		`
kubernetesVersion: v1.10.2
`}

	for _, testCase := range testCases {
		confBody := singleAzConfigYaml + testCase
		c, _ := ClusterFromBytes([]byte(confBody))
		if enabled, err := c.ApiServerLeaseEndpointReconciler(); enabled == false || err != nil {
			t.Errorf("API server lease endpoint should be enabled at Kubernetes 1.9 or greater: %s\n%s", err, confBody)
		}
	}
}

func TestKube2IamKiamClash(t *testing.T) {
	config := `
experimental:
  kube2IamSupport:
    enabled: true
  kiamSupport:
    enabled: true
`
	confBody := singleAzConfigYaml + config
	_, err := ClusterFromBytes([]byte(confBody))
	if err == nil || !strings.Contains(err.Error(), "not both") {
		t.Errorf("expected config to cause error as kube2iam and kiam cannot be enabled together: %s\n%s", err, confBody)
	}
}
func TestKMSArnValidateRegion(t *testing.T) {
	config := `keyName: test-key-name
s3URI: s3://mybucket/mydir
region: us-west-1
clusterName: test-cluster-name
kmsKeyArn: "arn:aws:kms:eu-west-1:xxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx"
`
	confBody := config + externalDNSNameConfig + availabilityZoneConfig

	_, err := ClusterFromBytes([]byte(confBody))
	if err == nil || !strings.Contains(err.Error(), "same region") {
		t.Errorf("Expecting validation error for mismatching KMS key ARN and region config: %s\n%s", err, confBody)
	}
}
func TestClusterAutoscalerDisabled(t *testing.T) {
	disabledConfigs := []string{
		`
addons:
  clusterAutoscaler:
    enabled: false
experimental:
  clusterAutoscalerSupport:
    enabled: true
`,
		`
addons:
  clusterAutoscaler:
    enabled: true
experimental:
  clusterAutoscalerSupport:
    enabled: false
`}

	for _, testCase := range disabledConfigs {
		confBody := singleAzConfigYaml + testCase
		c, err := ClusterFromBytes([]byte(confBody))
		if err != nil {
			t.Errorf("failed to parse config %s: %v", confBody, err)
		}

		for label, _ := range c.NodeLabels() {
			if label == "kube-aws.coreos.com/cluster-autoscaler-supported" {
				t.Errorf("Controllers should not be labelled for autoscaler with config: %s", confBody)
			}
		}

		if c.ClusterAutoscalerSupportEnabled() {
			t.Errorf("Controllers should not have autoscaling enabled with config: %s", confBody)
		}
	}
}

func TestClusterAutoscalerEnabled(t *testing.T) {
	enabledConfigs := []string{
		`
addons:
  clusterAutoscaler:
    enabled: true
experimental:
  clusterAutoscalerSupport:
    enabled: true
`,
		// `experimental.clusterAutoscalerSupport.enabled` should default to true
		`
addons:
  clusterAutoscaler:
    enabled: true
`}

	for _, testCase := range enabledConfigs {
		confBody := singleAzConfigYaml + testCase
		c, err := ClusterFromBytes([]byte(confBody))
		if err != nil {
			t.Errorf("failed to parse config %s: %v", confBody, err)
		}

		found := false
		for label, _ := range c.NodeLabels() {
			if label == "kube-aws.coreos.com/cluster-autoscaler-supported" {
				found = true
			}
		}
		if !found {
			t.Errorf("Controllers should be labelled for autoscaler with config: %s", confBody)
		}

		if !c.ClusterAutoscalerSupportEnabled() {
			t.Errorf("Controllers should have autoscaling enabled with config: %s", confBody)
		}
	}
}
