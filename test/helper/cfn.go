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

// DummyCFInterrogator is used to prevent calls to AWS - always returns empty results.
type DummyCFInterrogator struct {
	ListStacksResult          *cloudformation.ListStacksOutput
	ListStacksResourcesResult *cloudformation.ListStackResourcesOutput
}

func (cf DummyCFInterrogator) ListStacks(input *cloudformation.ListStacksInput) (*cloudformation.ListStacksOutput, error) {
	return cf.ListStacksResult, nil
}

func (cf DummyCFInterrogator) ListStackResources(input *cloudformation.ListStackResourcesInput) (*cloudformation.ListStackResourcesOutput, error) {
	return cf.ListStacksResourcesResult, nil
}

type DummyEC2Interrogator struct {
	DescribeInstancesOutput *ec2.DescribeInstancesOutput
}

func (ec DummyEC2Interrogator) DescribeInstances(input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	return ec.DescribeInstancesOutput, nil
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
