package cfnstack

import (
	"github.com/aws/aws-sdk-go/service/ec2"
)

type EC2Interrogator interface {
	DescribeSubnets(input *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error)
}
