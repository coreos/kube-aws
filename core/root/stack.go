package root

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/kubernetes-incubator/kube-aws/pkg/model"
	"io/ioutil"
	"strings"
)

func getStackTemplate(cfnSvc model.StackTemplateGetter, stackName string) (string, error) {
	byRootStackName := &cloudformation.GetTemplateInput{StackName: aws.String(stackName)}
	output, err := cfnSvc.GetTemplate(byRootStackName)
	if err != nil {
		return "", fmt.Errorf("failed to get current root stack template: %v", err)
	}
	return aws.StringValue(output.TemplateBody), nil
}

type StackResourceDescriber interface {
	DescribeStackResource(input *cloudformation.DescribeStackResourceInput) (*cloudformation.DescribeStackResourceOutput, error)
}

func getNestedStackName(cfnSvc StackResourceDescriber, stackName string, nestedStackLogicalName string) (string, error) {
	byRootStackName := &cloudformation.DescribeStackResourceInput{StackName: aws.String(stackName), LogicalResourceId: aws.String(nestedStackLogicalName)}
	output, err := cfnSvc.DescribeStackResource(byRootStackName)
	if err != nil {
		return "", fmt.Errorf("failed to get current root stack template: %v", err)
	}
	return aws.StringValue(output.StackResourceDetail.PhysicalResourceId), nil
}

func getInstanceScriptUserdata(stackJson string, nestedStackLogicalName string) (string, error) {
	dest := map[string]interface{}{}
	err := json.Unmarshal([]byte(stackJson), &dest)
	if err != nil {
		return "", err
	}
	res := dest["Resources"].(map[string]interface{})
	lc := res[nestedStackLogicalName].(map[string]interface{})
	props := lc["Properties"].(map[string]interface{})
	ud := props["UserData"].(map[string]interface{})
	fnBase64 := ud["Fn::Base64"].(map[string]interface{})
	fnJoin := fnBase64["Fn::Join"].([]interface{})
	joinedItems := fnJoin[1].([]interface{})
	instanceScript := joinedItems[3].(string)
	return instanceScript, nil
}

func getInstanceUserdataJson(stackJson string, nestedStackLogicalName string) (string, error) {
	dest := map[string]interface{}{}
	err := json.Unmarshal([]byte(stackJson), &dest)
	if err != nil {
		return "", err
	}
	res := dest["Resources"].(map[string]interface{})
	lc := res[nestedStackLogicalName].(map[string]interface{})
	props := lc["Properties"].(map[string]interface{})
	ud := props["UserData"].(map[string]interface{})
	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)
	// Avoid diffs like this:
	// -           "Fn::Sub": "echo 'KUBE_AWS_STACK_NAME=${AWS::StackName}' \u003e\u003e/var/run/coreos/etcd-node.env"
	// +           "Fn::Sub": "echo 'KUBE_AWS_STACK_NAME=${AWS::StackName}' >>/var/run/coreos/etcd-node.env"
	encoder.SetEscapeHTML(false)
	err = encoder.Encode(ud)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func getS3Userdata(s3Svc *s3.S3, instanceUserdata string) (string, error) {
	a := strings.Split(instanceUserdata, " cp ")
	b := strings.Split(a[1], " ")[0]
	s3uri := b
	tokens := strings.SplitN(strings.Split(s3uri, "s3://")[1], "/", 2)
	bucket := tokens[0]
	key := tokens[1]
	out, err := s3Svc.GetObject(&s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return "", err
	}
	bytes, err := ioutil.ReadAll(out.Body)
	if err != nil {
		return "", err
	}
	out.Body.Close()
	return string(bytes), nil
}
