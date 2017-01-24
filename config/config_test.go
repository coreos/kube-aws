package config

import (
	"bytes"
	"fmt"
	"net"
	"reflect"
	"strings"
	"testing"
	"text/template"

	model "github.com/coreos/kube-aws/model"
	"github.com/coreos/kube-aws/netutil"
	"gopkg.in/yaml.v2"
)

const minimalConfigYaml = `externalDNSName: test.staging.core-os.net
keyName: test-key-name
region: us-west-1
clusterName: test-cluster-name
kmsKeyArn: "arn:aws:kms:us-west-1:xxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx"
`

const availabilityZoneConfig = `
availabilityZone: us-west-1c
`

const singleAzConfigYaml = minimalConfigYaml + availabilityZoneConfig

var goodNetworkingConfigs = []string{
	``, //Tests validity of default network config values
	`
vpcCIDR: 10.4.3.0/24
instanceCIDR: 10.4.3.0/24
controllerIP: 10.4.3.5
podCIDR: 172.4.0.0/16
serviceCIDR: 172.5.0.0/16
dnsServiceIP: 172.5.100.101
`, `
vpcCIDR: 10.4.0.0/16
instanceCIDR: 10.4.3.0/24
controllerIP: 10.4.3.5
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
hostedZone: ""
`, `
createRecordSet: true
recordSetTTL: 400
hostedZone: core-os.net
`, `
createRecordSet: true
hostedZone: "staging.core-os.net"
`, `
createRecordSet: true
hostedZoneId: "XXXXXXXXXXX"
`,
}

var incorrectNetworkingConfigs = []string{
	`
vpcCIDR: 10.4.2.0/23
instanceCIDR: 10.4.3.0/24
controllerIP: 10.4.3.5
podCIDR: 10.4.0.0/16 #podCIDR contains vpcCDIR.
serviceCIDR: 10.5.0.0/16
dnsServiceIP: 10.5.100.101
`,
	`
vpcCIDR: 10.4.2.0/23
instanceCIDR: 10.4.3.0/24
controllerIP: 10.4.3.5
podCIDR: 10.5.0.0/16
serviceCIDR: 10.4.0.0/16 #serviceCIDR contains vpcCDIR.
dnsServiceIP: 10.4.100.101
`, `
vpcCIDR: 10.4.0.0/16
instanceCIDR: 10.5.3.0/24 #instanceCIDR not in vpcCIDR
controllerIP: 10.5.3.5
podCIDR: 10.6.0.0/16
serviceCIDR: 10.5.0.0/16
dnsServiceIP: 10.5.100.101
`, `
vpcCIDR: 10.4.3.0/16
instanceCIDR: 10.4.3.0/24
controllerIP: 10.4.3.5
podCIDR: 172.4.0.0/16
serviceCIDR: 172.5.0.0/16
dnsServiceIP: 172.5.0.1 #dnsServiceIP conflicts with kubernetesServiceIP
`, `
vpcCIDR: 10.4.3.0/16
instanceCIDR: 10.4.3.0/24
controllerIP: 10.4.3.5
podCIDR: 10.4.0.0/16 #vpcCIDR overlaps with podCIDR
serviceCIDR: 172.5.0.0/16
dnsServiceIP: 172.5.100.101

`, `
vpcCIDR: 10.4.3.0/16
instanceCIDR: 10.4.3.0/24
controllerIP: 10.4.3.5
podCIDR: 172.4.0.0/16
serviceCIDR: 172.5.0.0/16
dnsServiceIP: 172.6.100.101 #dnsServiceIP not in service CIDR
`, `
routeTableId: rtb-xxxxxx # routeTableId specified without vpcId
`, `
# invalid TTL
recordSetTTL: 0
`, `
# hostedZone and hostedZoneID shouldn't be blank when createRecordSet is true
createRecordSet: true
`, `
# recordSetTTL shouldn't be modified when createRecordSet is false
createRecordSet: false
recordSetTTL: 400
`, `
createRecordSet: true
recordSetTTL: 60
hostedZone: staging.core-os.net
hostedZoneId: /hostedzone/staging_id_2 #hostedZone and hostedZoneId defined
`,
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
releaseChannel: non-existant #this release channel will never exist
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
controllerIP: 10.4.3.50
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
		subnets []*model.Subnet
	}{
		{
			conf: `
# You can specify multiple subnets to be created in order to achieve H/A
vpcCIDR: 10.4.3.0/16
controllerIP: 10.4.3.50
subnets:
  - availabilityZone: ap-northeast-1a
    instanceCIDR: 10.4.3.0/24
  - availabilityZone: ap-northeast-1c
    instanceCIDR: 10.4.4.0/24
`,
			subnets: []*model.Subnet{
				{
					InstanceCIDR:     "10.4.3.0/24",
					AvailabilityZone: "ap-northeast-1a",
					CustomName:       "Subnet0",
					TopLevel:         true,
				},
				{
					InstanceCIDR:     "10.4.4.0/24",
					AvailabilityZone: "ap-northeast-1c",
					CustomName:       "Subnet1",
					TopLevel:         true,
				},
			},
		},
		{
			conf: `
# Given AZ/CIDR, missing subnets fall-back to the single subnet with the AZ/CIDR given.
vpcCIDR: 10.4.3.0/16
controllerIP: 10.4.3.50
availabilityZone: ap-northeast-1a
instanceCIDR: 10.4.3.0/24
`,
			subnets: []*model.Subnet{
				{
					AvailabilityZone: "ap-northeast-1a",
					InstanceCIDR:     "10.4.3.0/24",
					CustomName:       "Subnet0",
					TopLevel:         true,
				},
			},
		},
		{
			conf: `
# Given AZ/CIDR, empty subnets fall-back to the single subnet with the AZ/CIDR given.
vpcCIDR: 10.4.3.0/16
controllerIP: 10.4.3.50
availabilityZone: ap-northeast-1a
instanceCIDR: 10.4.3.0/24
subnets: []
`,
			subnets: []*model.Subnet{
				{
					AvailabilityZone: "ap-northeast-1a",
					InstanceCIDR:     "10.4.3.0/24",
					CustomName:       "Subnet0",
					TopLevel:         true,
				},
			},
		},
		{
			conf: `
# Given no AZ/CIDR, empty subnets fall-backs to the single subnet with the default az/cidr.
availabilityZone: "ap-northeast-1a"
subnets: []
`,
			subnets: []*model.Subnet{
				{
					AvailabilityZone: "ap-northeast-1a",
					InstanceCIDR:     "10.0.0.0/24",
					CustomName:       "Subnet0",
					TopLevel:         true,
				},
			},
		},
		{
			conf: `
# Missing subnets field fall-backs to the single subnet with the default az/cidr.
availabilityZone: "ap-northeast-1a"
`,
			subnets: []*model.Subnet{
				{
					AvailabilityZone: "ap-northeast-1a",
					InstanceCIDR:     "10.0.0.0/24",
					CustomName:       "Subnet0",
					TopLevel:         true,
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
# Both AZ/instanceCIDR is given. This is O.K. but...
- availabilityZone: "ap-northeast-1a"
# instanceCIDR does not include the default controllerIP
- instanceCIDR: 10.0.5.0/24
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
				"parsed subnets %s does not match expected subnets %s in config: %s",
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
controllerRootVolumeType: gp2
`,
			volumeType: "gp2",
			iops:       0,
		},
		{
			conf: `
controllerRootVolumeType: standard
`,
			volumeType: "standard",
			iops:       0,
		},
		{
			conf: `
controllerRootVolumeType: io1
controllerRootVolumeIOPS: 100
`,
			volumeType: "io1",
			iops:       100,
		},
		{
			conf: `
controllerRootVolumeType: io1
controllerRootVolumeIOPS: 2000
`,
			volumeType: "io1",
			iops:       2000,
		},
	}

	invalidConfigs := []string{
		`
# There's no volume type 'default'
controllerRootVolumeType: default
`,
		`
# IOPS must be zero for volume types != 'io1'
controllerRootVolumeType: standard
controllerRootVolumeIOPS: 100
`,
		`
# IOPS must be zero for volume types != 'io1'
controllerRootVolumeType: gp2
controllerRootVolumeIOPS: 2000
`,
		`
# IOPS smaller than the minimum (100)
controllerRootVolumeType: io1
controllerRootVolumeIOPS: 99
`,
		`
# IOPS greater than the maximum (2000)
controllerRootVolumeType: io1
controllerRootVolumeIOPS: 2001
`,
	}

	for _, conf := range validConfigs {
		confBody := singleAzConfigYaml + conf.conf
		c, err := ClusterFromBytes([]byte(confBody))
		if err != nil {
			t.Errorf("failed to parse config %s: %v", confBody, err)
			continue
		}
		if c.ControllerRootVolumeType != conf.volumeType {
			t.Errorf(
				"parsed root volume type %s does not match root volume %s in config: %s",
				c.ControllerRootVolumeType,
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
workerRootVolumeIOPS: 2000
`,
			volumeType: "io1",
			iops:       2000,
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
workerRootVolumeIOPS: 2000
`,
		`
# IOPS smaller than the minimum (100)
workerRootVolumeType: io1
workerRootVolumeIOPS: 99
`,
		`
# IOPS greater than the maximum (2000)
workerRootVolumeType: io1
workerRootVolumeIOPS: 2001
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
		nodeDrainer NodeDrainer
	}{
		{
			conf: `
`,
			nodeDrainer: NodeDrainer{
				Enabled: false,
			},
		},
		{
			conf: `
experimental:
  nodeDrainer:
    enabled: false
`,
			nodeDrainer: NodeDrainer{
				Enabled: false,
			},
		},
		{
			conf: `
experimental:
  nodeDrainer:
    enabled: true
`,
			nodeDrainer: NodeDrainer{
				Enabled: true,
			},
		},
		{
			conf: `
# Settings for an experimental feature must be under the "experimental" field. Ignored.
nodeDrainer:
  enabled: true
`,
			nodeDrainer: NodeDrainer{
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
		if !reflect.DeepEqual(c.Experimental.NodeDrainer, conf.nodeDrainer) {
			t.Errorf(
				"parsed node drainer settings %+v does not match config: %s",
				c.Experimental.NodeDrainer,
				confBody,
			)
		}
	}

}

func newMinimalConfig() (*Config, error) {
	cluster := NewDefaultCluster()
	cluster.ExternalDNSName = "k8s.example.com"
	cluster.Region = "us-west-1"
	cluster.Subnets = []*model.Subnet{
		&model.Subnet{
			AvailabilityZone: "us-west-1a",
			InstanceCIDR:     "10.0.0.0/24",
		},
		&model.Subnet{
			AvailabilityZone: "us-west-1b",
			InstanceCIDR:     "10.0.1.0/24",
		},
	}
	c, err := cluster.Config()
	if err != nil {
		return nil, err
	}
	c.TLSConfig = &CompactTLSAssets{
		CACert:         "examplecacert",
		CAKey:          "examplecakey",
		APIServerCert:  "exampleapiservercert",
		APIServerKey:   "exampleapiserverkey",
		WorkerCert:     "exampleworkercert",
		WorkerKey:      "exampleworkerkey",
		AdminCert:      "exampleadmincert",
		AdminKey:       "exampleadminkey",
		EtcdCert:       "exampleetcdcert",
		EtcdClientCert: "exampleetcdclientcert",
		EtcdClientKey:  "exampleetcdclientkey",
		EtcdKey:        "exampleetcdkey",
	}
	return c, nil
}

func renderTemplate(name string, templateBody []byte, data interface{}) (string, error) {
	tmpl, err := template.New(name).Parse(string(templateBody))
	if err != nil {
		return "", err
	}
	var buff bytes.Buffer
	if err := tmpl.Execute(&buff, data); err != nil {
		return "", err
	}
	return buff.String(), nil
}

func renderCloudConfigWorker(data interface{}) (string, error) {
	return renderTemplate("cluod-config-worker", _CloudConfigWorker, data)
}

func parseYaml(text string) (map[interface{}]interface{}, error) {
	parsedYaml := make(map[interface{}]interface{})
	err := yaml.Unmarshal([]byte(text), &parsedYaml)
	return parsedYaml, err
}

func TestNodeDrainerWorkerUserData(t *testing.T) {
	config, err := newMinimalConfig()

	if err != nil {
		t.Errorf("Unexpected error while setting up a test data: %s", err)
	}

	var cloudConfig string

	config.Experimental.NodeDrainer.Enabled = true
	if cloudConfig, err = renderCloudConfigWorker(config); err != nil {
		t.Errorf("failed to render worker cloud config: %v", err)
	}
	if _, err = parseYaml(cloudConfig); err != nil {
		t.Errorf("failed to parse as YAML %s: %v", cloudConfig, err)
	}
	if !strings.Contains(cloudConfig, "kube-node-drainer.service") {
		t.Errorf("expected \"kube-node-drainer.service\" to exist, but it didn't in the template output: %s", cloudConfig)

	}

	config.Cluster.Experimental.NodeDrainer.Enabled = false
	if cloudConfig, err = renderCloudConfigWorker(config); err != nil {
		t.Errorf("failed to render worker cloud config: %v", err)
	}
	if _, err = parseYaml(cloudConfig); err != nil {
		t.Errorf("Failed to parse as YAML %s: %v", cloudConfig, err)
	}
	if strings.Contains(cloudConfig, "kube-node-drainer.service") {
		t.Errorf("expected \"kube-node-drainer.service\" not to exist, but it did exist in the template output: %s", cloudConfig)

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
		cluster, err := ClusterFromBytes([]byte(confBody))
		if err != nil {
			t.Errorf("failed to parse config %s: %v", confBody, err)
		}

		_, err2 := cluster.Config()
		if err2 != nil {
			t.Errorf("failed to generate config for %s: %v", channel, err2)
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

	cluster := NewDefaultCluster()

	cluster.VPCCIDR = "10.0.0.0/16"
	cluster.Subnets = []*model.Subnet{
		{"ap-northeast-1a", "10.0.1.0/24", "", "", "", model.NatGateway{}, true},
		{"ap-northeast-1a", "10.0.2.0/24", "", "", "", model.NatGateway{}, true},
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

func TestWithTrailingDot(t *testing.T) {
	tests := [][]string{
		[]string{
			"",
			"",
		},
		[]string{
			"foo.bar.",
			"foo.bar.",
		},
		[]string{
			"foo.bar",
			"foo.bar.",
		},
	}

	for _, test := range tests {
		input := test[0]
		actual := WithTrailingDot(input)
		expected := test[1]

		if expected != actual {
			t.Errorf(
				"WithTrailingDot(\"%s\") expected to return \"%s\" but it returned: \"%s\"",
				input,
				expected,
				actual,
			)
		}
	}
}
