package cfnstack

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/kubernetes-incubator/kube-aws/logger"
)

var CFN_TEMPLATE_SIZE_LIMIT = 51200

type CreationService interface {
	CreateStack(*cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error)
}

type UpdateService interface {
	UpdateStack(input *cloudformation.UpdateStackInput) (*cloudformation.UpdateStackOutput, error)
}

type CRUDService interface {
	CreateStack(*cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error)
	UpdateStack(input *cloudformation.UpdateStackInput) (*cloudformation.UpdateStackOutput, error)
	DescribeStacks(input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error)
	DescribeStackEvents(input *cloudformation.DescribeStackEventsInput) (*cloudformation.DescribeStackEventsOutput, error)
	EstimateTemplateCost(input *cloudformation.EstimateTemplateCostInput) (*cloudformation.EstimateTemplateCostOutput, error)
}

type S3ObjectPutterService interface {
	PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error)
}

type cfInterrogator interface {
	ListStacks(input *cloudformation.ListStacksInput) (*cloudformation.ListStacksOutput, error)
	ListStackResources(input *cloudformation.ListStackResourcesInput) (*cloudformation.ListStackResourcesOutput, error)
}

func StackEventErrMsgs(events []*cloudformation.StackEvent) []string {
	var errMsgs []string

	for _, event := range events {
		if aws.StringValue(event.ResourceStatus) == cloudformation.ResourceStatusCreateFailed {
			// Only show actual failures, not cancelled dependent resources.
			if aws.StringValue(event.ResourceStatusReason) != "Resource creation cancelled" {
				errMsgs = append(errMsgs,
					strings.TrimSpace(
						strings.Join([]string{
							aws.StringValue(event.ResourceStatus),
							aws.StringValue(event.ResourceType),
							aws.StringValue(event.LogicalResourceId),
							aws.StringValue(event.ResourceStatusReason),
						}, " ")))
			}
		}
	}

	return errMsgs
}

func NestedStackExists(cf cfInterrogator, parentStackName, stackName string) (bool, error) {
	logger.Debugf("Testing whether nested stack '%s' is present in parent stack '%s'", stackName, parentStackName)
	parentExists, err := StackExists(cf, parentStackName)
	if err != nil {
		return false, err
	}
	if !parentExists {
		logger.Debugf("Parent stack '%s' does not exist, so nested stack can not exist either", parentStackName)
		return false, nil
	}

	req := &cloudformation.ListStackResourcesInput{StackName: &parentStackName}
	logger.Debugf("Calling AWS cloudformation ListStackResources for stack %s ->", parentStackName)
	out, err := cf.ListStackResources(req)
	if err != nil {
		return false, fmt.Errorf("Could not read cf stack %s: %v", parentStackName, err)
	}
	logger.Debugf("<- AWS responded with %d stack resources", len(out.StackResourceSummaries))
	for _, resource := range out.StackResourceSummaries {
		if *resource.LogicalResourceId == stackName {
			logger.Debugf("Match! resource id '%s' exists", stackName)
			return true, nil
		}
	}
	logger.Debugf("No match! resource id '%s' does not exist", stackName)
	return false, nil
}

func StackExists(cf cfInterrogator, stackName string) (bool, error) {
	logger.Debugf("Testing whether cf stack %s exits", stackName)
	req := &cloudformation.ListStacksInput{}
	logger.Debug("Calling AWS cloudformation ListStacks ->")
	stacks, err := cf.ListStacks(req)
	if err != nil {
		return false, fmt.Errorf("Could not list cloudformation stacks: %v", err)
	}
	logger.Debugf("<- AWS Responded with %d stacks", len(stacks.StackSummaries))
	for _, summary := range stacks.StackSummaries {
		if *summary.StackName == stackName {
			logger.Debugf("Found matching stack %s: %+v", *summary.StackName, *summary)
			if summary.DeletionTime == nil {
				logger.Debugf("Stack is active - matched!")
				return true, nil
			} else {
				logger.Debugf("Stack is not active, ignoring")
			}
		}
	}
	logger.Debugf("Found no active stacks with id %s", stackName)
	return false, nil
}
