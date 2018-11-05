package cfnstack

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStackCreationErrorMessaging(t *testing.T) {
	events := []*cloudformation.StackEvent{
		&cloudformation.StackEvent{
			// Failure with all fields set
			ResourceStatus:       aws.String("CREATE_FAILED"),
			ResourceType:         aws.String("Computer"),
			LogicalResourceId:    aws.String("test_comp"),
			ResourceStatusReason: aws.String("BAD HD"),
		},
		&cloudformation.StackEvent{
			// Success, should not show up
			ResourceStatus: aws.String("SUCCESS"),
			ResourceType:   aws.String("Computer"),
		},
		&cloudformation.StackEvent{
			// Failure due to cancellation should not show up
			ResourceStatus:       aws.String("CREATE_FAILED"),
			ResourceType:         aws.String("Computer"),
			ResourceStatusReason: aws.String("Resource creation cancelled"),
		},
		&cloudformation.StackEvent{
			// Failure with missing fields
			ResourceStatus: aws.String("CREATE_FAILED"),
			ResourceType:   aws.String("Computer"),
		},
	}

	expectedMsgs := []string{
		"CREATE_FAILED Computer test_comp BAD HD",
		"CREATE_FAILED Computer",
	}

	outputMsgs := StackEventErrMsgs(events)
	if len(expectedMsgs) != len(outputMsgs) {
		t.Errorf("Expected %d stack error messages, got %d\n",
			len(expectedMsgs),
			len(StackEventErrMsgs(events)))
	}

	for i := range expectedMsgs {
		if expectedMsgs[i] != outputMsgs[i] {
			t.Errorf("Expected `%s`, got `%s`\n", expectedMsgs[i], outputMsgs[i])
		}
	}
}

// DummyCFInterrogator is used to prevent calls to AWS - always returns empty results.
type DummyCFInterrogator struct {
	ListStacksResourcesResult *cloudformation.ListStackResourcesOutput
	DescribeStacksResult      *cloudformation.DescribeStacksOutput
}

func (cf DummyCFInterrogator) ListStackResources(input *cloudformation.ListStackResourcesInput) (*cloudformation.ListStackResourcesOutput, error) {
	return cf.ListStacksResourcesResult, nil
}

func (cf DummyCFInterrogator) DescribeStacks(input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
	return cf.DescribeStacksResult, nil
}

func TestStackDoesNotExist(t *testing.T) {
	var stackname = "the-little-stack-on-the-prairie"
	var summary = cloudformation.Stack{
		StackName:    &stackname,
		DeletionTime: nil,
	}
	cf := DummyCFInterrogator{
		DescribeStacksResult: &cloudformation.DescribeStacksOutput{
			Stacks: []*cloudformation.Stack{&summary},
		},
	}
	exists, err := StackExists(cf, "Elvis")

	require.NoError(t, err, "Looking up aws stacks should not fail when mocked out")
	assert.False(t, exists, "StackExists thinks that the stack 'Elvis' exists, even though no stacks were returned")
}

func TestStackDoesExist(t *testing.T) {
	var stackname = "the-little-stack-on-the-prairie"
	var summary = cloudformation.Stack{
		StackName:    &stackname,
		DeletionTime: nil,
	}
	cf := DummyCFInterrogator{
		DescribeStacksResult: &cloudformation.DescribeStacksOutput{
			Stacks: []*cloudformation.Stack{&summary},
		},
	}

	exists, err := StackExists(cf, "the-little-stack-on-the-prairie")
	require.NoError(t, err, "Looking up aws stacks should not fail when mocked out")
	assert.True(t, exists, "The response includes a non deleted stack and so we should get a positive exists")
}

func TestStackExistsIgnoresDeletedStacks(t *testing.T) {
	testtime := time.Now()
	var stackname = "the-little-stack-on-the-prairie"
	var summary = cloudformation.Stack{
		StackName:    &stackname,
		DeletionTime: &testtime,
	}
	cf := DummyCFInterrogator{
		DescribeStacksResult: &cloudformation.DescribeStacksOutput{
			Stacks: []*cloudformation.Stack{&summary, &summary},
		},
	}

	exists, err := StackExists(cf, "the-little-stack-on-the-prairie")
	require.NoError(t, err, "Looking up aws stacks should not fail when mocked out")
	assert.False(t, exists, "We should only return true if an active/not-deleted stack is found")
}

func TestStackExistsWithMultipleNameMatches(t *testing.T) {
	testtime := time.Now()
	var stackname = "the-little-stack-on-the-prairie"
	var deletedstack = cloudformation.Stack{
		StackName:    &stackname,
		DeletionTime: &testtime,
	}
	var activestack = cloudformation.Stack{
		StackName:    &stackname,
		DeletionTime: nil,
	}
	cf := DummyCFInterrogator{
		DescribeStacksResult: &cloudformation.DescribeStacksOutput{
			Stacks: []*cloudformation.Stack{&deletedstack, &deletedstack, &deletedstack, &activestack},
		},
	}

	exists, err := StackExists(cf, "the-little-stack-on-the-prairie")
	require.NoError(t, err, "Looking up aws stacks should not fail when mocked out")
	assert.True(t, exists, "An active stack exists and so we should be returning true")
}

func TestANestedCantExistWithoutItsParent(t *testing.T) {
	var stackname = "the-little-stack-on-the-prairie"
	var summary = cloudformation.Stack{
		StackName:    &stackname,
		DeletionTime: nil,
	}
	cf := DummyCFInterrogator{
		DescribeStacksResult: &cloudformation.DescribeStacksOutput{
			Stacks: []*cloudformation.Stack{&summary},
		},
	}

	exists, err := NestedStackExists(cf, "Elvis", "the-little-stack-on-the-prairie")
	require.NoError(t, err, "NestedStackExists should not return an error when the parent does not exist")
	assert.False(t, exists, "A stack can't exist as a nested stack unless the parent exists and has a resource reference to it")
}

func TestANestedStackHasToBeAResourceOfItsParent(t *testing.T) {
	var stackname = "the-little-stack-on-the-prairie"
	var parentname = "root-stack"
	var summary = cloudformation.Stack{
		StackName:    &stackname,
		DeletionTime: nil,
	}
	var parentsummary = cloudformation.Stack{
		StackName:    &parentname,
		DeletionTime: nil,
	}
	cf := DummyCFInterrogator{
		DescribeStacksResult: &cloudformation.DescribeStacksOutput{
			Stacks: []*cloudformation.Stack{&parentsummary, &summary},
		},
		ListStacksResourcesResult: &cloudformation.ListStackResourcesOutput{
			StackResourceSummaries: []*cloudformation.StackResourceSummary{},
		},
	}

	exists, err := NestedStackExists(cf, "root-stack", "the-little-stack-on-the-prairie")
	require.NoError(t, err, "NestedStackExists should not return an error")
	assert.False(t, exists, "Although the parent exists it does not include a stack resource with our name (even if a top level stack shares the same name as us)")
}

func TestNestedStackExists(t *testing.T) {
	var stackname = "the-little-stack-on-the-prairie"
	var parentname = "root-stack"
	var summary = cloudformation.Stack{
		StackName:    &stackname,
		DeletionTime: nil,
	}
	var parentsummary = cloudformation.Stack{
		StackName:    &parentname,
		DeletionTime: nil,
	}
	var parentresources = cloudformation.StackResourceSummary{
		LogicalResourceId: &stackname,
	}
	cf := DummyCFInterrogator{
		DescribeStacksResult: &cloudformation.DescribeStacksOutput{
			Stacks: []*cloudformation.Stack{&parentsummary, &summary},
		},
		ListStacksResourcesResult: &cloudformation.ListStackResourcesOutput{
			StackResourceSummaries: []*cloudformation.StackResourceSummary{&parentresources},
		},
	}

	exists, err := NestedStackExists(cf, "root-stack", "the-little-stack-on-the-prairie")
	require.NoError(t, err, "NestedStackExists should not return an error")
	assert.True(t, exists, "A stack that is as a resource within its parent stack exists")
}

func TestEmptyCFInterrogator(t *testing.T) {
	cf := DummyCFInterrogator{}

	exists, err := StackExists(cf, "a-new-hope")
	require.NoError(t, err, "StackExists should not return an error when the DescribeStacks response is empty")
	assert.False(t, exists, "How does a stack exist in an empty set?")
	exists, err = NestedStackExists(cf, "no-hope", "a-new-hope")
	require.NoError(t, err, "NestedStackExists should not return an error when the ListStackResources response is empty")
	assert.False(t, exists, "How does a stack exist in an empty set?")
}
