package cfnstack

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/s3"
	"regexp"
	"strings"
	"time"
)

type Provisioner struct {
	stackName       string
	stackTags       map[string]string
	stackPolicyBody string
	session         *session.Session
	s3URI           string
}

func NewProvisioner(name string, stackTags map[string]string, s3URI string, stackPolicyBody string, session *session.Session) *Provisioner {
	return &Provisioner{
		stackName:       name,
		stackTags:       stackTags,
		stackPolicyBody: stackPolicyBody,
		session:         session,
		s3URI:           s3URI,
	}
}

func (c *Provisioner) uploadFile(s3Svc S3ObjectPutterService, stackBody string, filename string) (string, error) {
	s3URI := c.s3URI

	re := regexp.MustCompile("s3://(?P<bucket>[^/]+)/(?P<directory>.+[^/])/*$")
	matches := re.FindStringSubmatch(s3URI)

	var bucket string
	var key string
	if len(matches) == 3 {
		directory := matches[2]

		bucket = matches[1]
		key = fmt.Sprintf("%s/%s/%s", directory, c.stackName, filename)
	} else {
		re := regexp.MustCompile("s3://(?P<bucket>[^/]+)/*$")
		matches := re.FindStringSubmatch(s3URI)

		if len(matches) == 2 {
			bucket = matches[1]
			key = fmt.Sprintf("%s/%s", c.stackName, filename)
		} else {
			return "", fmt.Errorf("failed to parse s3 uri(=%s): The valid uri pattern for it is s3://mybucket/mydir or s3://mybucket", s3URI)
		}
	}

	contentLength := int64(len(stackBody))
	body := strings.NewReader(stackBody)

	_, err := s3Svc.PutObject(&s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(contentLength),
		ContentType:   aws.String("application/json"),
	})

	if err != nil {
		return "", err
	}

	uploadedFileURL := fmt.Sprintf("https://s3.amazonaws.com/%s/%s", bucket, key)

	return uploadedFileURL, nil
}

func (c *Provisioner) uploadStackAssets(s3Svc S3ObjectPutterService, stackTemplate string, cloudConfigs map[string]string) (*string, error) {
	templateURL, err := c.uploadFile(s3Svc, stackTemplate, "stack.json")
	if err != nil {
		return nil, fmt.Errorf("Template uplaod failed: %v", err)
	}

	for filename, content := range cloudConfigs {
		if _, err := c.uploadFile(s3Svc, content, filename); err != nil {
			return nil, fmt.Errorf("File upload failed: %v", err)
		}
	}

	return &templateURL, nil
}

func (c *Provisioner) CreateStack(cfSvc CreationService, s3Svc S3ObjectPutterService, stackTemplate string, cloudConfigs map[string]string) (*cloudformation.CreateStackOutput, error) {
	templateURL, uploadErr := c.uploadStackAssets(s3Svc, stackTemplate, cloudConfigs)

	if uploadErr != nil {
		return nil, fmt.Errorf("template upload failed: %v", uploadErr)
	} else if templateURL != nil {
		resp, err := c.createStackFromTemplateURL(cfSvc, *templateURL)
		if err != nil {
			return nil, fmt.Errorf("stack creation failed: %v", err)
		}

		return resp, nil
	} else {
		return nil, fmt.Errorf("[bug] kube-aws skipped template upload")
	}
}

func (c *Provisioner) CreateStackAndWait(cfSvc CRUDService, s3Svc S3ObjectPutterService, stackTemplate string, cloudConfigs map[string]string) error {
	resp, err := c.CreateStack(cfSvc, s3Svc, stackTemplate, cloudConfigs)
	if err != nil {
		return err
	}

	req := cloudformation.DescribeStacksInput{
		StackName: resp.StackId,
	}

	for {
		resp, err := cfSvc.DescribeStacks(&req)
		if err != nil {
			return err
		}
		if len(resp.Stacks) == 0 {
			return fmt.Errorf("stack not found")
		}
		statusString := aws.StringValue(resp.Stacks[0].StackStatus)
		switch statusString {
		case cloudformation.ResourceStatusCreateComplete:
			return nil
		case cloudformation.ResourceStatusCreateFailed:
			errMsg := fmt.Sprintf(
				"Stack creation failed: %s : %s",
				statusString,
				aws.StringValue(resp.Stacks[0].StackStatusReason),
			)
			errMsg = errMsg + "\n\nPrinting the most recent failed stack events:\n"

			stackEventsOutput, err := cfSvc.DescribeStackEvents(
				&cloudformation.DescribeStackEventsInput{
					StackName: resp.Stacks[0].StackName,
				})
			if err != nil {
				return err
			}
			errMsg = errMsg + strings.Join(StackEventErrMsgs(stackEventsOutput.StackEvents), "\n")
			return errors.New(errMsg)
		case cloudformation.ResourceStatusCreateInProgress:
			time.Sleep(3 * time.Second)
			continue
		default:
			return fmt.Errorf("unexpected stack status: %s", statusString)
		}
	}
}

func (c *Provisioner) baseCreateStackInput() *cloudformation.CreateStackInput {
	var tags []*cloudformation.Tag
	for k, v := range c.stackTags {
		key := k
		value := v
		tags = append(tags, &cloudformation.Tag{Key: &key, Value: &value})
	}

	return &cloudformation.CreateStackInput{
		StackName:       aws.String(c.stackName),
		OnFailure:       aws.String(cloudformation.OnFailureDoNothing),
		Capabilities:    []*string{aws.String(cloudformation.CapabilityCapabilityIam)},
		Tags:            tags,
		StackPolicyBody: aws.String(c.stackPolicyBody),
	}
}

func (c *Provisioner) createStackFromTemplateURL(cfSvc CreationService, stackTemplateURL string) (*cloudformation.CreateStackOutput, error) {
	input := c.baseCreateStackInput()
	input.TemplateURL = &stackTemplateURL
	return cfSvc.CreateStack(input)
}

func (c *Provisioner) baseUpdateStackInput() *cloudformation.UpdateStackInput {
	return &cloudformation.UpdateStackInput{
		Capabilities: []*string{aws.String(cloudformation.CapabilityCapabilityIam)},
		StackName:    aws.String(c.stackName),
	}
}

func (c *Provisioner) updateStackWithTemplateURL(cfSvc UpdateService, templateURL string) (*cloudformation.UpdateStackOutput, error) {
	input := c.baseUpdateStackInput()
	input.TemplateURL = aws.String(templateURL)
	return cfSvc.UpdateStack(input)
}

func (c *Provisioner) UpdateStack(cfSvc UpdateService, s3Svc S3ObjectPutterService, stackTemplate string, cloudConfigs map[string]string) (*cloudformation.UpdateStackOutput, error) {
	templateURL, uploadErr := c.uploadStackAssets(s3Svc, stackTemplate, cloudConfigs)

	if uploadErr != nil {
		return nil, fmt.Errorf("template upload failed: %v", uploadErr)
	} else if templateURL != nil {
		resp, err := c.updateStackWithTemplateURL(cfSvc, *templateURL)
		if err != nil {
			return nil, fmt.Errorf("stack update failed: %v", err)
		}

		return resp, nil
	} else {
		return nil, fmt.Errorf("[bug] kube-aws skipped template upload")
	}
}

func (c *Provisioner) UpdateStackAndWait(cfSvc CRUDService, s3Svc S3ObjectPutterService, stackTemplate string, cloudConfigs map[string]string) (string, error) {
	updateOutput, err := c.UpdateStack(cfSvc, s3Svc, stackTemplate, cloudConfigs)
	if err != nil {
		return "", fmt.Errorf("error updating cloudformation stack: %v", err)
	}
	req := cloudformation.DescribeStacksInput{
		StackName: updateOutput.StackId,
	}
	for {
		resp, err := cfSvc.DescribeStacks(&req)
		if err != nil {
			return "", err
		}
		if len(resp.Stacks) == 0 {
			return "", fmt.Errorf("stack not found")
		}
		statusString := aws.StringValue(resp.Stacks[0].StackStatus)
		switch statusString {
		case cloudformation.ResourceStatusUpdateComplete:
			return updateOutput.String(), nil
		case cloudformation.ResourceStatusUpdateFailed, cloudformation.StackStatusUpdateRollbackComplete, cloudformation.StackStatusUpdateRollbackFailed:
			errMsg := fmt.Sprintf("Stack status: %s : %s", statusString, aws.StringValue(resp.Stacks[0].StackStatusReason))
			return "", errors.New(errMsg)
		case cloudformation.ResourceStatusUpdateInProgress, cloudformation.StackStatusUpdateCompleteCleanupInProgress:
			time.Sleep(3 * time.Second)
			continue
		default:
			return "", fmt.Errorf("unexpected stack status: %s", statusString)
		}
	}
}

func (c *Provisioner) Validate(stackBody string) (string, error) {
	validateInput := cloudformation.ValidateTemplateInput{}

	templateURL, uploadErr := c.uploadStackAssets(s3.New(c.session), stackBody, map[string]string{})

	if uploadErr != nil {
		return "", fmt.Errorf("template upload failed: %v", uploadErr)
	} else if templateURL != nil {
		validateInput.TemplateURL = templateURL
	} else {
		return "", fmt.Errorf("[bug] kube-aws skipped template upload")
	}

	cfSvc := cloudformation.New(c.session)
	validationReport, err := cfSvc.ValidateTemplate(&validateInput)
	if err != nil {
		return "", fmt.Errorf("invalid cloudformation stack: %v", err)
	}

	return validationReport.String(), nil
}

type Destroyer struct {
	stackName string
	session   *session.Session
}

func NewDestroyer(stackName string, session *session.Session) *Destroyer {
	return &Destroyer{
		stackName: stackName,
		session:   session,
	}
}

func (c *Destroyer) Destroy() error {
	cfSvc := cloudformation.New(c.session)
	dreq := &cloudformation.DeleteStackInput{
		StackName: aws.String(c.stackName),
	}
	_, err := cfSvc.DeleteStack(dreq)
	return err
}
