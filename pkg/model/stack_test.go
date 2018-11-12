package model

import (
	"github.com/kubernetes-incubator/kube-aws/test/helper"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/stretchr/testify/assert"

	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-yaml/yaml"
	"github.com/kubernetes-incubator/kube-aws/credential"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"github.com/kubernetes-incubator/kube-aws/plugin/clusterextension"
	"os"
	"path/filepath"
)

/*
TODO(colhom): when we fully deprecate instanceCIDR/availabilityZone, this block of
logic will go away and be replaced by a single constant string
*/
func genConfigYamlForTesting(configYaml string) (string, error) {
	defaultYaml := `
externalDNSName: test.staging.core-os.net
keyName: test-key-name
s3URI: s3://mybucket/mydir
region: us-west-1
clusterName: test-cluster-name
kmsKeyArn: "arn:aws:kms:us-west-1:xxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx"
`
	yamlStr := defaultYaml + configYaml

	c := api.Cluster{}
	if err := yaml.Unmarshal([]byte(yamlStr), &c); err != nil {
		return "", fmt.Errorf("failed umarshalling config yaml: %v :\n%s", err, yamlStr)
	}

	if len(c.Subnets) > 0 {
		for i := range c.Subnets {
			c.Subnets[i].AvailabilityZone = fmt.Sprintf("dummy-az-%d", i)
		}
	} else {
		//Legacy behavior
		c.AvailabilityZone = "dummy-az-0"
	}

	out, err := yaml.Marshal(&c)
	if err != nil {
		return "", fmt.Errorf("error marshalling cluster: %v", err)
	}

	return string(out), nil
}

type VPC struct {
	cidr        string
	subnetCidrs []string
}

type dummyEC2Service struct {
	VPCs               map[string]VPC
	KeyPairs           map[string]bool
	ExpectedRootVolume *ec2.CreateVolumeInput
}

func (svc dummyEC2Service) CreateVolume(input *ec2.CreateVolumeInput) (*ec2.Volume, error) {
	expected := svc.ExpectedRootVolume

	if !aws.BoolValue(input.DryRun) {
		return nil, fmt.Errorf(
			"expected dry-run request to create volume endpoint, but DryRun was false",
		)
	}

	if aws.StringValue(input.AvailabilityZone) != "dummy-az-0" {
		return nil, fmt.Errorf(
			"expected AvailabilityZone to be %v, but was %v",
			"dummy-az-0",
			aws.StringValue(input.AvailabilityZone),
		)
	}

	if aws.Int64Value(input.Iops) != aws.Int64Value(expected.Iops) {
		return nil, fmt.Errorf(
			"unexpected root volume iops\nexpected=%v, observed=%v",
			aws.Int64Value(expected.Iops),
			aws.Int64Value(input.Iops),
		)
	}

	if aws.Int64Value(input.Size) != aws.Int64Value(expected.Size) {
		return nil, fmt.Errorf(
			"unexpected root volume size\nexpected=%v, observed=%v",
			aws.Int64Value(expected.Size),
			aws.Int64Value(input.Size),
		)
	}

	if aws.StringValue(input.VolumeType) != aws.StringValue(expected.VolumeType) {
		return nil, fmt.Errorf(
			"unexpected root volume type\nexpected=%v, observed=%v",
			aws.StringValue(expected.VolumeType),
			aws.StringValue(input.VolumeType),
		)
	}

	return nil, nil
}

func (svc dummyEC2Service) DescribeVpcs(input *ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error) {
	output := ec2.DescribeVpcsOutput{}
	for _, vpcID := range input.VpcIds {
		if vpc, ok := svc.VPCs[*vpcID]; ok {
			output.Vpcs = append(output.Vpcs, &ec2.Vpc{
				VpcId:     vpcID,
				CidrBlock: aws.String(vpc.cidr),
			})
		}
	}

	return &output, nil
}

func (svc dummyEC2Service) DescribeSubnets(input *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error) {
	output := ec2.DescribeSubnetsOutput{}

	var vpcIds []string
	for _, filter := range input.Filters {
		if *filter.Name == "vpc-id" {
			for _, value := range filter.Values {
				vpcIds = append(vpcIds, *value)
			}
		}
	}

	for _, vpcID := range vpcIds {
		if vpc, ok := svc.VPCs[vpcID]; ok {
			for _, subnetCidr := range vpc.subnetCidrs {
				output.Subnets = append(
					output.Subnets,
					&ec2.Subnet{CidrBlock: aws.String(subnetCidr)},
				)
			}
		}
	}

	return &output, nil
}

func (svc dummyEC2Service) DescribeKeyPairs(input *ec2.DescribeKeyPairsInput) (*ec2.DescribeKeyPairsOutput, error) {
	output := &ec2.DescribeKeyPairsOutput{}

	for _, keyName := range input.KeyNames {
		if _, ok := svc.KeyPairs[*keyName]; ok {
			output.KeyPairs = append(output.KeyPairs, &ec2.KeyPairInfo{
				KeyName: keyName,
			})
		} else {
			return nil, awserr.New("InvalidKeyPair.NotFound", "", errors.New(""))
		}
	}

	return output, nil
}

func TestExistingVPCValidation(t *testing.T) {

	goodExistingVPCConfigs := []string{
		``, //Tests default create VPC mode, which bypasses existing VPC validation
		`
vpcCIDR: 10.5.0.0/16
vpcId: vpc-xxx1
subnets:
- name: Subnet0
  routeTable:
    id: rtb-xxxxxx
  instanceCIDR: 10.5.11.0/24
`, `
vpcCIDR: 192.168.1.0/24
vpcId: vpc-xxx2
internetGatewayId: igw-xxx1
instanceCIDR: 192.168.1.50/28
`, `
vpcCIDR: 192.168.1.0/24
vpcId: vpc-xxx2
internetGatewayId: igw-xxx1
subnets:
  - instanceCIDR: 192.168.1.0/28
  - instanceCIDR: 192.168.1.32/28
  - instanceCIDR: 192.168.1.64/28
`,
	}

	badExistingVPCConfigs := []string{
		`
vpcCIDR: 10.0.0.0/16
vpcId: vpc-xxx3 #vpc does not exist
subnets:
- name: Subnet0
  routeTable:
    id: rtb-xxxxxx
  instanceCIDR: 10.0.0.0/24
`, `
vpcCIDR: 10.10.0.0/16 #vpc cidr does match existing vpc-xxx1
vpcId: vpc-xxx1
subnets:
- name: Subnet0
  routeTable:
    id: rtb-xxxxxx
  instanceCIDR: 10.10.0.0/24
`, `
vpcCIDR: 10.5.0.0/16
instanceCIDR: 10.5.2.0/28 #instance cidr conflicts with existing subnet
vpcId: vpc-xxx1
`, `
vpcCIDR: 192.168.1.0/24
instanceCIDR: 192.168.1.100/26 #instance cidr conflicts with existing subnet
vpcId: vpc-xxx2
`, `
vpcCIDR: 192.168.1.0/24
vpcId: vpc-xxx2
subnets:
  - instanceCIDR: 192.168.1.100/26  #instance cidr conflicts with existing subnet
    routeTable:
      id: rtb-xxxxxx
  - instanceCIDR: 192.168.1.0/26
    routeTable:
      id: rtb-xxxxxx
`,
	}

	ec2Service := dummyEC2Service{
		VPCs: map[string]VPC{
			"vpc-xxx1": {
				cidr: "10.5.0.0/16",
				subnetCidrs: []string{
					"10.5.1.0/24",
					"10.5.2.0/24",
					"10.5.10.100/29",
				},
			},
			"vpc-xxx2": {
				cidr: "192.168.1.0/24",
				subnetCidrs: []string{
					"192.168.1.100/28",
					"192.168.1.150/28",
					"192.168.1.200/28",
				},
			},
		},
	}

	validateCluster := func(networkConfig string) error {
		cluster, err := newStackRefForTesting(networkConfig)
		if err != nil {
			t.Errorf("could not get valid cluster config: %v\n%s", err, networkConfig)
			return nil
		}

		return cluster.validateExistingVPCState(ec2Service)
	}

	for _, networkConfig := range goodExistingVPCConfigs {
		if err := validateCluster(networkConfig); err != nil {
			t.Errorf("Correct config tested invalid: %s\n%s", err, networkConfig)
		}
	}

	for _, networkConfig := range badExistingVPCConfigs {
		if err := validateCluster(networkConfig); err == nil {
			t.Errorf("Incorrect config tested valid, expected error:\n%s", networkConfig)
		}
	}
}

func TestValidateKeyPair(t *testing.T) {
	conf, err := genConfigYamlForTesting("")
	if err != nil {
		t.Errorf("%v", err)
	}

	clusterConfig, err := ClusterFromBytes([]byte(conf))
	if err != nil {
		t.Errorf("could not get valid cluster config: %v", err)
	}

	c := &StackRef{Cluster: clusterConfig}

	ec2Svc := dummyEC2Service{}
	ec2Svc.KeyPairs = map[string]bool{
		c.KeyName: true,
	}

	if err := c.validateKeyPair(ec2Svc); err != nil {
		t.Errorf("returned an error for valid key")
	}

	c.KeyName = "invalidKeyName"
	if err := c.validateKeyPair(ec2Svc); err == nil {
		t.Errorf("failed to catch invalid key \"%s\"", c.KeyName)
	}
}

type Zone struct {
	Id  string
	DNS string
}

type dummyR53Service struct {
	HostedZones        []Zone
	ResourceRecordSets map[string]string
}

func (r53 dummyR53Service) ListHostedZonesByName(input *route53.ListHostedZonesByNameInput) (*route53.ListHostedZonesByNameOutput, error) {
	output := &route53.ListHostedZonesByNameOutput{}
	for _, zone := range r53.HostedZones {
		if zone.DNS == WithTrailingDot(aws.StringValue(input.DNSName)) {
			output.HostedZones = append(output.HostedZones, &route53.HostedZone{
				Name: aws.String(zone.DNS),
				Id:   aws.String(zone.Id),
			})
		}
	}
	return output, nil
}

func (r53 dummyR53Service) ListResourceRecordSets(input *route53.ListResourceRecordSetsInput) (*route53.ListResourceRecordSetsOutput, error) {
	output := &route53.ListResourceRecordSetsOutput{}
	if name, ok := r53.ResourceRecordSets[aws.StringValue(input.HostedZoneId)]; ok {
		output.ResourceRecordSets = []*route53.ResourceRecordSet{
			&route53.ResourceRecordSet{
				Name: aws.String(name),
			},
		}
	}
	return output, nil
}

func (r53 dummyR53Service) GetHostedZone(input *route53.GetHostedZoneInput) (*route53.GetHostedZoneOutput, error) {
	for _, zone := range r53.HostedZones {
		if zone.Id == aws.StringValue(input.Id) {
			return &route53.GetHostedZoneOutput{
				HostedZone: &route53.HostedZone{
					Id:   aws.String(zone.Id),
					Name: aws.String(zone.DNS),
				},
			}, nil
		}
	}
	return nil, fmt.Errorf("dummy route53 service: no hosted zone with id '%s'", aws.StringValue(input.Id))
}

func TestValidateDNSConfig(t *testing.T) {
	r53 := dummyR53Service{
		HostedZones: []Zone{
			{
				Id:  "/hostedzone/staging_id_1",
				DNS: "staging.core-os.net.",
			},
			{
				Id:  "/hostedzone/staging_id_2",
				DNS: "staging.core-os.net.",
			},
			{
				Id:  "/hostedzone/staging_id_3",
				DNS: "zebras.coreos.com.",
			},
			{
				Id:  "/hostedzone/staging_id_4",
				DNS: "core-os.net.",
			},
		},
		ResourceRecordSets: map[string]string{
			"staging_id_1": "existing-record.staging.core-os.net.",
		},
	}

	validDNSConfigs := []string{
		`
createRecordSet: true
recordSetTTL: 60
hostedZoneId: staging_id_1
`, `
createRecordSet: true
recordSetTTL: 60
hostedZoneId: /hostedzone/staging_id_2
`,
	}

	invalidDNSConfigs := []string{
		`
createRecordSet: true
recordSetTTL: 60
hostedZoneId: /hostedzone/staging_id_3 # <staging_id_id> is not a super-domain
`, `
createRecordSet: true
recordSetTTL: 60
hostedZoneId: /hostedzone/staging_id_5 #non-existent hostedZoneId
`,
	}

	for _, validConfig := range validDNSConfigs {
		configBody, err := genConfigYamlForTesting(validConfig)
		if err != nil {
			t.Errorf("%v", err)
			return
		}
		clusterConfig, err := ClusterFromBytes([]byte(configBody))
		if err != nil {
			t.Errorf("could not get valid cluster config in `%s`: %v", configBody, err)
			continue
		}
		c := &StackRef{Cluster: clusterConfig}

		if err := c.validateDNSConfig(r53); err != nil {
			t.Errorf("returned error for valid config: %v", err)
		}
	}

	for _, invalidConfig := range invalidDNSConfigs {
		configBody, err := genConfigYamlForTesting(invalidConfig)
		if err != nil {
			t.Errorf("%v", err)
			return
		}
		clusterConfig, err := ClusterFromBytes([]byte(configBody))
		if err != nil {
			t.Errorf("could not get valid cluster config: %v", err)
			continue
		}
		c := &StackRef{Cluster: clusterConfig}

		if err := c.validateDNSConfig(r53); err == nil {
			t.Errorf("failed to produce error for invalid config: %s", configBody)
		}
	}

}

func TestStackTags(t *testing.T) {
	testCases := []struct {
		expectedTags []*cloudformation.Tag
		clusterYaml  string
	}{
		{
			expectedTags: []*cloudformation.Tag{},
			clusterYaml: `
#no stackTags set
`,
		},
		{
			expectedTags: []*cloudformation.Tag{
				&cloudformation.Tag{
					Key:   aws.String("KeyA"),
					Value: aws.String("ValueA"),
				},
				&cloudformation.Tag{
					Key:   aws.String("KeyB"),
					Value: aws.String("ValueB"),
				},
				&cloudformation.Tag{
					Key:   aws.String("KeyC"),
					Value: aws.String("ValueC"),
				},
			},
			clusterYaml: `
stackTags:
  KeyA: ValueA
  KeyB: ValueB
  KeyC: ValueC
`,
		},
	}

	pwd, err := os.Getwd()
	if err != nil {
		t.Errorf("%v", err)
		t.FailNow()
	}
	for _, testCase := range testCases {
		helper.WithDummyCredentials(func(dummyAssetsDir string) {
			var stackTemplateOptions = api.StackTemplateOptions{
				AssetsDir:             dummyAssetsDir,
				ControllerTmplFile:    filepath.Join(pwd, "../../builtin/files/userdata/cloud-config-controller"),
				EtcdTmplFile:          filepath.Join(pwd, "../../builtin/files/userdata/cloud-config-etcd"),
				StackTemplateTmplFile: filepath.Join(pwd, "../../builtin/files/stack-templates/control-plane.json.tmpl"),
			}

			stack, err := newStackForTesting(testCase.clusterYaml, stackTemplateOptions)
			if !assert.NoError(t, err) {
				return
			}

			assets, err := stack.buildAssets()
			if !assert.NoError(t, err) {
				return
			}

			userdataFilename := ""
			var asset api.Asset
			var id api.AssetID
			for id, asset = range assets.AsMap() {
				if strings.HasPrefix(id.Filename, "userdata-controller-") {
					userdataFilename = id.Filename
					break
				}
			}
			assert.NotZero(t, userdataFilename, "Unable to find userdata-controller asset")

			path, err := asset.S3Prefix()
			assert.NoError(t, err)
			assert.Equal(t, "mybucket/mydir/kube-aws/clusters/test-cluster-name/exported/stacks/control-plane/userdata-controller", path, "UserDataController.S3Prefix returned an unexpected value")
		})
	}
}

func TestIsSubdomain(t *testing.T) {
	validData := []struct {
		sub    string
		parent string
	}{
		{
			// single level
			sub:    "test.coreos.com",
			parent: "coreos.com",
		},
		{
			// multiple levels
			sub:    "cgag.staging.coreos.com",
			parent: "coreos.com",
		},
		{
			// trailing dots shouldn't matter
			sub:    "staging.coreos.com.",
			parent: "coreos.com.",
		},
		{
			// trailing dots shouldn't matter
			sub:    "a.b.c.",
			parent: "b.c",
		},
		{
			// multiple level parent domain
			sub:    "a.b.c.staging.core-os.net",
			parent: "staging.core-os.net",
		},
	}

	invalidData := []struct {
		sub    string
		parent string
	}{
		{
			// mismatch
			sub:    "staging.coreos.com",
			parent: "example.com",
		},
		{
			// superdomain is longer than subdomain
			sub:    "staging.coreos.com",
			parent: "cgag.staging.coreos.com",
		},
	}

	for _, valid := range validData {
		if !isSubdomain(valid.sub, valid.parent) {
			t.Errorf("%s should be a valid subdomain of %s", valid.sub, valid.parent)
		}
	}

	for _, invalid := range invalidData {
		if isSubdomain(invalid.sub, invalid.parent) {
			t.Errorf("%s should not be a valid subdomain of %s", invalid.sub, invalid.parent)
		}
	}

}

func TestValidateControllerRootVolume(t *testing.T) {
	testCases := []struct {
		expectedRootVolume *ec2.CreateVolumeInput
		clusterYaml        string
	}{
		{
			expectedRootVolume: &ec2.CreateVolumeInput{
				Iops:       aws.Int64(0),
				Size:       aws.Int64(30),
				VolumeType: aws.String("gp2"),
			},
			clusterYaml: `
# no root volumes set
`,
		},
		{
			expectedRootVolume: &ec2.CreateVolumeInput{
				Iops:       aws.Int64(0),
				Size:       aws.Int64(30),
				VolumeType: aws.String("standard"),
			},
			clusterYaml: `
controller:
  rootVolume:
    type: standard
`,
		},
		{
			expectedRootVolume: &ec2.CreateVolumeInput{
				Iops:       aws.Int64(0),
				Size:       aws.Int64(50),
				VolumeType: aws.String("gp2"),
			},
			clusterYaml: `
controller:
  rootVolume:
    type: gp2
    size: 50
`,
		},
		{
			expectedRootVolume: &ec2.CreateVolumeInput{
				Iops:       aws.Int64(20000),
				Size:       aws.Int64(100),
				VolumeType: aws.String("io1"),
			},
			clusterYaml: `
controller:
  rootVolume:
    type: io1
    size: 100
    iops: 20000
`,
		},
		// TODO Remove test cases for deprecated keys in v0.9.7
		{
			expectedRootVolume: &ec2.CreateVolumeInput{
				Iops:       aws.Int64(0),
				Size:       aws.Int64(30),
				VolumeType: aws.String("standard"),
			},
			clusterYaml: `
controllerRootVolumeType: standard
`,
		},
		{
			expectedRootVolume: &ec2.CreateVolumeInput{
				Iops:       aws.Int64(0),
				Size:       aws.Int64(50),
				VolumeType: aws.String("gp2"),
			},
			clusterYaml: `
controllerRootVolumeType: gp2
controllerRootVolumeSize: 50
`,
		},
		{
			expectedRootVolume: &ec2.CreateVolumeInput{
				Iops:       aws.Int64(20000),
				Size:       aws.Int64(100),
				VolumeType: aws.String("io1"),
			},
			clusterYaml: `
controllerRootVolumeType: io1
controllerRootVolumeSize: 100
controllerRootVolumeIOPS: 20000
`,
		},
	}

	for _, testCase := range testCases {
		c, err := newStackRefForTesting(testCase.clusterYaml)
		if err != nil {
			t.Errorf("%v", err)
			return
		}

		ec2Svc := &dummyEC2Service{
			ExpectedRootVolume: testCase.expectedRootVolume,
		}

		if err := c.validateControllerRootVolume(ec2Svc); err != nil {
			t.Errorf("error creating cluster: %v\nfor test case %+v", err, testCase)
		}
	}
}

func newStackRefForTesting(yaml string) (*StackRef, error) {
	conf, err := genConfigYamlForTesting(yaml)
	if err != nil {
		return nil, err
	}

	c, err := ClusterFromBytes([]byte(conf))
	if err != nil {
		return nil, err
	}

	return &StackRef{Cluster: c}, nil
}

func newStackForTesting(yaml string, opts api.StackTemplateOptions) (*Stack, error) {
	conf, err := genConfigYamlForTesting(yaml)
	if err != nil {
		return nil, err
	}

	return yamlToStackForTesting(conf, opts)
}

func yamlToStackForTesting(yaml string, opts api.StackTemplateOptions) (*Stack, error) {
	c, err := ClusterFromBytes([]byte(yaml))
	if err != nil {
		return nil, err
	}

	return clusterToStackForTesting(c, opts)
}

func defaultStackForTesting(opts api.StackTemplateOptions) (*Stack, error) {
	c := api.NewDefaultCluster()
	c.HyperkubeImage.Tag = c.K8sVer

	c.Region = api.RegionForName("us-west-1")
	c.Subnets = []api.Subnet{
		api.NewPublicSubnet("us-west-1a", "10.0.1.0/24"),
		api.NewPublicSubnet("us-west-1b", "10.0.2.0/24"),
	}
	c.ExternalDNSName = "foo.example.com"
	c.KeyName = "mykey"
	c.S3URI = "s3://mybucket/mydir"
	c.KMSKeyARN = "arn:aws:kms:us-west-1:xxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx"

	if err := c.Load(); err != nil {
		return nil, err
	}

	return clusterToStackForTesting(c, opts)
}

func clusterToStackForTesting(c *api.Cluster, opts api.StackTemplateOptions) (*Stack, error) {
	compiled, err := Compile(c, api.ClusterOptions{})
	if err != nil {
		return nil, err
	}

	sess := &Context{
		ProvidedEncryptService: helper.DummyEncryptService{},
	}

	genopts := credential.GeneratorOptions{
		GenerateCA: true,
		KIAM:       true,
	}

	if _, err := sess.GenerateAssetsOnDisk(compiled, opts.AssetsDir, genopts); err != nil {
		return nil, fmt.Errorf("failed  generating default assets: %v", err)
	}

	assets, err := sess.LoadCredentials(compiled, opts)
	if err != nil {
		return nil, err
	}

	stack, err := NewControlPlaneStack(compiled, opts, clusterextension.NewExtras(), assets)
	if err != nil {
		return nil, err
	}

	return stack, nil
}

func TestRenderStackTemplate(t *testing.T) {
	pwd, err := os.Getwd()
	if err != nil {
		t.Errorf("%v", err)
		t.FailNow()
	}
	helper.WithDummyCredentials(func(dir string) {
		var stackTemplateOptions = api.StackTemplateOptions{
			AssetsDir:             dir,
			ControllerTmplFile:    filepath.Join(pwd, "../../builtin/files/userdata/cloud-config-controller"),
			EtcdTmplFile:          filepath.Join(pwd, "../../builtin/files/userdata/cloud-config-etcd"),
			StackTemplateTmplFile: filepath.Join(pwd, "../../builtin/files/stack-templates/control-plane.json.tmpl"),
		}
		stack, err := defaultStackForTesting(stackTemplateOptions)
		if assert.NoError(t, err, "Unable to initialize Cluster") {
			_, err = stack.RenderStackTemplateAsString()
			assert.NoError(t, err, "Unable to render stack template")
		}
	})
}
