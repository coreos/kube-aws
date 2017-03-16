package cluster

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	controlplane "github.com/coreos/kube-aws/core/controlplane/config"
	"github.com/coreos/kube-aws/core/nodepool/config"
	"github.com/coreos/kube-aws/test/helper"
)

type dummyEC2CreateVolumeService struct {
	ExpectedRootVolume *ec2.CreateVolumeInput
}

func (svc dummyEC2CreateVolumeService) CreateVolume(input *ec2.CreateVolumeInput) (*ec2.Volume, error) {
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

type dummyEC2DescribeKeyPairsService struct {
	KeyPairs map[string]bool
}

func (svc dummyEC2DescribeKeyPairsService) DescribeKeyPairs(input *ec2.DescribeKeyPairsInput) (*ec2.DescribeKeyPairsOutput, error) {
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

func TestValidateKeyPair(t *testing.T) {
	main, err := controlplane.ConfigFromBytes([]byte(`clusterName: test-cluster
externalDNSName: test-cluster.example.com
keyName: mykey
kmsKeyArn: mykeyarn
region: us-west-1
availabilityZone: us-west-1a
`))
	if err != nil {
		t.Errorf("[bug] failed to initialize test cluster : %v", err)
	}

	c, err := ClusterRefFromBytes([]byte(minimalYaml), main, false)
	if err != nil {
		t.Errorf("could not get valid cluster config: %v", err)
	}

	ec2Svc := dummyEC2DescribeKeyPairsService{}
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

const minimalYaml = `name: pool1
`

func TestValidateWorkerRootVolume(t *testing.T) {
	main, err := controlplane.ConfigFromBytes([]byte(`clusterName: test-cluster
externalDNSName: test-cluster.example.com
keyName: mykey
kmsKeyArn: mykeyarn
region: us-west-1
availabilityZone: dummy-az-0
`))
	if err != nil {
		t.Errorf("[bug] failed to initialize test cluster : %v", err)
	}

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
rootVolumeType: standard
`,
		},
		{
			expectedRootVolume: &ec2.CreateVolumeInput{
				Iops:       aws.Int64(0),
				Size:       aws.Int64(50),
				VolumeType: aws.String("gp2"),
			},
			clusterYaml: `
rootVolumeType: gp2
rootVolumeSize: 50
`,
		},
		{
			expectedRootVolume: &ec2.CreateVolumeInput{
				Iops:       aws.Int64(2000),
				Size:       aws.Int64(100),
				VolumeType: aws.String("io1"),
			},
			clusterYaml: `
rootVolumeType: io1
rootVolumeSize: 100
rootVolumeIOPS: 2000
`,
		},
	}

	for _, testCase := range testCases {
		configBody := minimalYaml + testCase.clusterYaml
		c, err := ClusterRefFromBytes([]byte(configBody), main, false)
		if err != nil {
			t.Errorf("failed to read cluster config: %v", err)
		}

		ec2Svc := &dummyEC2CreateVolumeService{
			ExpectedRootVolume: testCase.expectedRootVolume,
		}

		if err := c.validateWorkerRootVolume(ec2Svc); err != nil {
			t.Errorf("error creating cluster: %v\nfor test case %+v", err, testCase)
		}
	}
}

func TestStackUploadsAndCreation(t *testing.T) {
	mainConfigBody := `
externalDNSName: test.staging.core-os.net
keyName: test-key-name
region: us-west-1
clusterName: test-cluster-name
kmsKeyArn: "arn:aws:kms:us-west-1:xxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx"
availabilityZone: us-west-1a
`

	nodePoolConfigBody := `
name: pool1
`
	expectedTags := []*cloudformation.Tag{}

	main, err := controlplane.ConfigFromBytes([]byte(mainConfigBody))
	if err != nil {
		t.Errorf("failed to get valid cluster config: %v", err)
	}

	clusterConfig, err := config.ClusterFromBytesWithEncryptService([]byte(nodePoolConfigBody), main, helper.DummyEncryptService{})
	if err != nil {
		t.Errorf("could not get valid cluster config: %v", err)
	}

	cfSvc := &helper.DummyCloudformationService{
		ExpectedTags: expectedTags,
	}

	s3Svc := &helper.DummyS3ObjectPutterService{
		ExpectedBody:          "{}",
		ExpectedBucket:        "test-bucket",
		ExpectedContentType:   "application/json",
		ExpectedKey:           "foo/bar/kube-aws/clusters/test-cluster-name/exported/stacks/pool1/stack.json",
		ExpectedContentLength: 2,
	}

	helper.WithDummyCredentials(func(dummyAssetsDir string) {
		var stackTemplateOptions = config.StackTemplateOptions{
			AssetsDir:             dummyAssetsDir,
			StackTemplateTmplFile: "../config/templates/stack-template.json",
			WorkerTmplFile:        "../../controlplane/config/templates/cloud-config-worker",
			S3URI:                 "s3://test-bucket/foo/bar",
		}

		cluster, err := NewCluster(clusterConfig, stackTemplateOptions, false)
		if err != nil {
			t.Errorf("%v", err)
			t.FailNow()
		}

		_, err = cluster.stackProvisioner().CreateStack(cfSvc, s3Svc, "{}", map[string]string{})

		if err != nil {
			t.Errorf("error creating cluster: %v", err)
		}

		path, err := cluster.UserDataWorkerS3Path()
		if err != nil {
			t.Errorf("failed to get worker user data path in s3: %v", err)
		}

		if path != "test-bucket/foo/bar/kube-aws/clusters/test-cluster-name/exported/stacks/pool1/userdata-worker" {
			t.Errorf("UserDataControllerS3Path returned an unexpected value: %s", path)
		}
	})
}
