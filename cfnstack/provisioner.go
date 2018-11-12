package cfnstack

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
)

type Provisioner struct {
	stackName       string
	stackTags       map[string]string
	stackPolicyBody string
	session         *session.Session
	s3URI           string
	roleARN         string
	region          api.Region
}

func NewProvisioner(name string, stackTags map[string]string, s3URI string, region api.Region, stackPolicyBody string, session *session.Session, options ...string) *Provisioner {
	p := &Provisioner{
		stackName:       name,
		stackTags:       stackTags,
		stackPolicyBody: stackPolicyBody,
		session:         session,
		s3URI:           s3URI,
		region:          region,
	}

	if len(options) > 0 {
		roleARN := options[0]
		p.roleARN = roleARN
	}

	return p
}

func (c *Provisioner) uploadAsset(s3Svc S3ObjectPutterService, asset api.Asset) error {
	bucket := asset.Bucket
	key := asset.Key
	content := asset.Content
	contentLength := int64(len(content))
	body := strings.NewReader(content)

	_, err := s3Svc.PutObject(&s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(contentLength),
		ContentType:   aws.String("application/json"),
	})

	return err
}

func (c *Provisioner) UploadAssets(s3Svc S3ObjectPutterService, assets Assets) error {
	for _, a := range assets.AsMap() {
		err := c.uploadAsset(s3Svc, a)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Provisioner) EstimateTemplateCost(cfSvc CRUDService, body string, parameters []*cloudformation.Parameter) (*cloudformation.EstimateTemplateCostOutput, error) {

	input := cloudformation.EstimateTemplateCostInput{
		TemplateBody: &body,
		Parameters:   parameters,
	}
	templateCost, err := cfSvc.EstimateTemplateCost(&input)
	return templateCost, err
}

func (c *Provisioner) CreateStackAtURLAndWait(cfSvc CRUDService, templateURL string) error {
	resp, err := c.createStackFromTemplateURL(cfSvc, templateURL)
	if err != nil {
		return err
	}
	return c.waitUntilStackGetsCreated(cfSvc, resp)
}

func (c *Provisioner) waitUntilStackGetsCreated(cfSvc CRUDService, resp *cloudformation.CreateStackOutput) error {
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

	input := &cloudformation.CreateStackInput{
		StackName:       aws.String(c.stackName),
		OnFailure:       aws.String(cloudformation.OnFailureDoNothing),
		Capabilities:    []*string{aws.String(cloudformation.CapabilityCapabilityIam), aws.String(cloudformation.CapabilityCapabilityNamedIam)},
		Tags:            tags,
		StackPolicyBody: aws.String(c.stackPolicyBody),
	}

	if c.roleARN != "" {
		input = input.SetRoleARN(c.roleARN)
	}

	return input
}

func (c *Provisioner) createStackFromTemplateURL(cfSvc CreationService, stackTemplateURL string) (*cloudformation.CreateStackOutput, error) {
	input := c.baseCreateStackInput()
	input.TemplateURL = &stackTemplateURL
	return cfSvc.CreateStack(input)
}

func (c *Provisioner) baseUpdateStackInput() *cloudformation.UpdateStackInput {
	var tags []*cloudformation.Tag
	for k, v := range c.stackTags {
		key := k
		value := v
		tags = append(tags, &cloudformation.Tag{Key: &key, Value: &value})
	}

	input := &cloudformation.UpdateStackInput{
		Capabilities: []*string{aws.String(cloudformation.CapabilityCapabilityIam), aws.String(cloudformation.CapabilityCapabilityNamedIam)},
		StackName:    aws.String(c.stackName),
		Tags:         tags,
	}
	if c.roleARN != "" {
		input = input.SetRoleARN(c.roleARN)
	}
	return input
}

func (c *Provisioner) updateStackWithTemplateURL(cfSvc UpdateService, templateURL string) (*cloudformation.UpdateStackOutput, error) {
	input := c.baseUpdateStackInput()
	input.TemplateURL = aws.String(templateURL)
	return cfSvc.UpdateStack(input)
}

func (c *Provisioner) UpdateStackAtURLAndWait(cfSvc CRUDService, templateURL string) (string, error) {
	updateOutput, err := c.updateStackWithTemplateURL(cfSvc, templateURL)
	if err != nil {
		return "", fmt.Errorf("error updating cloudformation stack: %v", err)
	}
	return c.waitUntilStackGetsUpdated(cfSvc, updateOutput)
}

func (c *Provisioner) waitUntilStackGetsUpdated(cfSvc CRUDService, updateOutput *cloudformation.UpdateStackOutput) (string, error) {
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

func (c *Provisioner) ValidateStackAtURL(templateURL string) (string, error) {
	if templateURL == "" {
		return "", errors.New("[bug] ValidateStackAtURL: templateURL must not be nil")
	}

	validateInput := cloudformation.ValidateTemplateInput{
		TemplateURL: aws.String(templateURL),
	}

	cfSvc := cloudformation.New(c.session)
	validationReport, err := cfSvc.ValidateTemplate(&validateInput)
	if err != nil {
		return "", fmt.Errorf("invalid cloudformation stack template %s: %v", templateURL, err)
	}

	return validationReport.String(), nil
}

type Destroyer struct {
	roleARN   string
	stackName string
	session   *session.Session
}

func NewDestroyer(stackName string, session *session.Session, roleARN string) *Destroyer {
	return &Destroyer{
		stackName: stackName,
		session:   session,
		roleARN:   roleARN,
	}
}

func (c *Destroyer) Destroy() error {
	cfSvc := cloudformation.New(c.session)
	dreq := &cloudformation.DeleteStackInput{
		StackName: aws.String(c.stackName),
	}
	if c.roleARN != "" {
		dreq = dreq.SetRoleARN(c.roleARN)
	}
	_, err := cfSvc.DeleteStack(dreq)
	return err
}

func (c *Provisioner) StreamEventsNested(q chan struct{}, f *cloudformation.CloudFormation, stackId string, headStackName string, t time.Time) error {
	nestedStacks := make(map[string]bool)
	nestedQuit := make(chan struct{}, 1)
	var lastSeenEventId string
	defer func() { nestedQuit <- struct{}{} }()
	for {
		select {
		case <-q:
			return nil
		case <-time.After(1 * time.Second):
			events := make([]cloudformation.StackEvent, 0)

			_ = f.DescribeStackEventsPages(
				&cloudformation.DescribeStackEventsInput{StackName: &stackId},
				func(page *cloudformation.DescribeStackEventsOutput, lastPage bool) bool {
					for _, e := range page.StackEvents {
						if (e.Timestamp).Before(t) {
							return false
						}
						if *e.EventId == lastSeenEventId {
							return false
						}
						events = append(events, *e)
					}
					return true
				})

			for i := len(events) - 1; i >= 0; i-- {
				e := events[i]
				if *e.ResourceType == "AWS::CloudFormation::Stack" && *e.PhysicalResourceId != *e.StackId && !nestedStacks[*e.PhysicalResourceId] {
					nestedStacks[*e.PhysicalResourceId] = true
					go c.StreamEventsNested(nestedQuit, f, *e.PhysicalResourceId, headStackName, t)
				}
				eventPrettyPrint(e, headStackName, t)
				lastSeenEventId = *e.EventId
			}
		}
	}
}

func eventPrettyPrint(e cloudformation.StackEvent, n string, t time.Time) {
	ns := strings.Split(strings.TrimLeft(*e.StackName, n), "-")
	if len(ns) > 2 {
		n = "\t" + ns[len(ns)-2]
	} else {
		n = ""
	}

	s := int((*e.Timestamp).Sub(t).Seconds())
	d := fmt.Sprintf("+%.2d:%.2d:%.2d", s/3600, (s/60)%60, s%60)
	if e.ResourceStatusReason != nil {
		logger.Infof("%s%s\t%s\t\t%s\t\"%s\"\n", d, n, resize(*e.ResourceStatus, 24), resize(*e.LogicalResourceId, 22), *e.ResourceStatusReason)
	} else {
		logger.Infof("%s%s\t%s\t\t%s\n", d, n, resize(*e.ResourceStatus, 24), resize(*e.LogicalResourceId, 22))
	}
}

func resize(s string, i int) string {
	if len(s) < i {
		s += strings.Repeat(" ", i-len(s))
	}
	return s
}
