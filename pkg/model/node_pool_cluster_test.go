package model

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"

	"errors"
	"fmt"
	"testing"
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

	if (input.Iops == nil && expected.Iops != nil) ||
		(input.Iops != nil && expected.Iops == nil) ||
		aws.Int64Value(input.Iops) != aws.Int64Value(expected.Iops) {
		return nil, fmt.Errorf(
			"unexpected root volume iops\n raw values expected=%v, observed=%v \n "+
				"dereferenced values: expected=%v, observed=%v",
			expected.Iops,
			input.Iops,
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

func clusterRefFromBytes(bytes []byte) (*NodePoolStackRef, error) {
	provided, err := NodePoolConfigFromBytes(bytes)
	if err != nil {
		return nil, err
	}
	c := newNodePoolStackRef(provided, nil)
	return c, nil
}

func TestNodePoolStackValidateKeyPair(t *testing.T) {
	main := `clusterName: test-cluster
s3URI: s3://mybucket/mydir
apiEndpoints:
- name: public
  nodePoolRollingStrategy: parallel
  dnsName: test-cluster.example.com
  loadBalancer:
    hostedZone:
      id: hostedzone-xxxx
keyName: mykey
kmsKeyArn: arn:aws:kms:us-west-1:xxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx
region: us-west-1
availabilityZone: us-west-1a
`
	c, err := clusterRefFromBytes([]byte(main + minimalYaml))
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

const minimalYaml = `worker:
  nodePools:
  - name: pool1
`

func TestValidateWorkerRootVolume(t *testing.T) {
	m := `clusterName: test-cluster
s3URI: s3://mybucket/mydir
apiEndpoints:
- name: public
  dnsName: test-cluster.example.com
  loadBalancer:
    recordSetManaged: false
keyName: mykey
kmsKeyArn: arn:aws:kms:us-west-1:xxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx
region: us-west-1
availabilityZone: dummy-az-0
`
	testCases := []struct {
		expectedRootVolume *ec2.CreateVolumeInput
		clusterYaml        string
	}{
		{
			expectedRootVolume: &ec2.CreateVolumeInput{
				Size:       aws.Int64(30),
				VolumeType: aws.String("gp2"),
			},
			clusterYaml: `
# no root volumes set
`,
		},
		{
			expectedRootVolume: &ec2.CreateVolumeInput{
				Size:       aws.Int64(30),
				VolumeType: aws.String("standard"),
			},
			clusterYaml: `
    rootVolume:
      type: standard
`,
		},
		{
			expectedRootVolume: &ec2.CreateVolumeInput{
				Size:       aws.Int64(50),
				VolumeType: aws.String("gp2"),
			},
			clusterYaml: `
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
    rootVolume:
      type: io1
      size: 100
      iops: 20000
`,
		},
	}

	for _, testCase := range testCases {
		configBody := m + minimalYaml + testCase.clusterYaml
		c, err := clusterRefFromBytes([]byte(configBody))
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
