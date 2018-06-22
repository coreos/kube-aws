package cfnstack

import (
	"github.com/aws/aws-sdk-go/service/ec2"
)

type EC2Interrogator interface {
	DescribeInstances(input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error)
}
