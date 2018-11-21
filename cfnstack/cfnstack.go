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

// Used for querying existance of stacks and nested stacks.
type CFInterrogator interface {
	ListStackResources(input *cloudformation.ListStackResourcesInput) (*cloudformation.ListStackResourcesOutput, error)
	DescribeStacks(input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error)
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

func NestedStackExists(cf CFInterrogator, parentStackName, stackName string) (bool, error) {
	logger.Debugf("testing whether nested stack '%s' is present in parent stack '%s'", stackName, parentStackName)
	parentExists, err := StackExists(cf, parentStackName)
	if err != nil {
		return false, err
	}
	if !parentExists {
		logger.Debugf("parent stack '%s' does not exist, so nested stack can not exist either", parentStackName)
		return false, nil
	}

	req := &cloudformation.ListStackResourcesInput{StackName: &parentStackName}
	logger.Debugf("calling AWS cloudformation ListStackResources for stack %s ->", parentStackName)
	out, err := cf.ListStackResources(req)
	if err != nil {
		return false, fmt.Errorf("Could not read cf stack %s: %v", parentStackName, err)
	}
	if out == nil {
		return false, nil
	}
	logger.Debugf("<- AWS responded with %d stack resources", len(out.StackResourceSummaries))
	for _, resource := range out.StackResourceSummaries {
		if *resource.LogicalResourceId == stackName {
			logger.Debugf("match! resource id '%s' exists", stackName)
			return true, nil
		}
	}
	logger.Debugf("no match! resource id '%s' does not exist", stackName)
	return false, nil
}

func StackExists(cf CFInterrogator, stackName string) (bool, error) {
	logger.Debugf("testing whether cf stack %s exists", stackName)
	req := &cloudformation.DescribeStacksInput{}
	req.StackName = &stackName
	logger.Debug("calling AWS cloudformation DescribeStacks ->")
	stacks, err := cf.DescribeStacks(req)
	if err != nil {
		if strings.HasPrefix(err.Error(), "ValidationError: Stack with id "+stackName+" does not exist") {
			return false, nil
		}
		return false, fmt.Errorf("could not list cloudformation stacks: %v", err)
	}
	if stacks == nil {
		logger.Debugf("<- AWS Responded with empty stacks object")
		return false, nil
	}

	if stacks.Stacks != nil {
		logger.Debugf("<- AWS Responded with %d stacks", len(stacks.Stacks))
		for _, summary := range stacks.Stacks {
			if *summary.StackName == stackName {
				logger.Debugf("found matching stack %s: %+v", *summary.StackName, *summary)
				if summary.DeletionTime == nil {
					logger.Debugf("stack is active - matched!")
					return true, nil
				} else {
					logger.Debugf("stack is not active, ignoring")
				}
			}
		}
	}
	logger.Debugf("found no active stacks with id %s", stackName)
	return false, nil
}
