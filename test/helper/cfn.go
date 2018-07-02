package helper

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type DummyCloudformationService struct {
	ExpectedTags []*cloudformation.Tag
	StackEvents  []*cloudformation.StackEvent
	StackStatus  string
}

type DummyEC2Interrogator struct {
	DescribeSubnetsOutput *ec2.DescribeSubnetsOutput
}

func (ec DummyEC2Interrogator) DescribeSubnets(input *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error) {
	return ec.DescribeSubnetsOutput, nil
}

func (cfSvc *DummyCloudformationService) CreateStack(req *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {

	if len(cfSvc.ExpectedTags) != len(req.Tags) {
		return nil, fmt.Errorf(
			"expected tag count does not match supplied tag count\nexpected=%v, supplied=%v",
			cfSvc.ExpectedTags,
			req.Tags,
		)
	}

	matchCnt := 0
	for _, eTag := range cfSvc.ExpectedTags {
		for _, tag := range req.Tags {
			if *tag.Key == *eTag.Key && *tag.Value == *eTag.Value {
				matchCnt++
				break
			}
		}
	}

	if matchCnt != len(cfSvc.ExpectedTags) {
		return nil, fmt.Errorf(
			"not all tags matched\nexpected=%v, observed=%v",
			cfSvc.ExpectedTags,
			req.Tags,
		)
	}

	resp := &cloudformation.CreateStackOutput{
		StackId: req.StackName,
	}

	return resp, nil
}
