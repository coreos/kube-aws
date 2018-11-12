package api

import (
	"encoding/json"
	"fmt"
)

type ARN struct {
	Arn                string `yaml:"arn,omitempty"`
	ArnFromStackOutput string `yaml:"arnFromStackOutput,omitempty"`
	ArnFromFn          string `yaml:"arnFromFn,omitempty"`
}

// HasArn returns true when the id of a resource i.e. either `arn` or `arnFromStackOutput` is specified
func (i ARN) HasArn() bool {
	return i.Arn != "" || i.ArnFromStackOutput != ""
}

func (i ARN) Validate() error {
	if i.ArnFromFn != "" {
		var jsonHolder map[string]interface{}
		if err := json.Unmarshal([]byte(i.ArnFromFn), &jsonHolder); err != nil {
			return fmt.Errorf("arnFromFn must be a valid json expression but was not: %s", i.ArnFromFn)
		}
	}
	return nil
}

func (i ARN) OrGetAttArn(logicalNameProvider func() (string, error)) (string, error) {
	return i.OrExpr(func() (string, error) {
		logicalName, err := logicalNameProvider()
		if err != nil {
			// So that kube-aws can print a more useful error message with
			// the line number for the stack-template.json when there's an error
			return "", fmt.Errorf("failed to get arn: %v", err)
		}
		return fmt.Sprintf(`{ "Fn::GetAtt": [ %q, "Arn" ] }`, logicalName), nil
	})
}

func (i ARN) OrRef(logicalNameProvider func() (string, error)) (string, error) {
	return i.OrExpr(func() (string, error) {
		logicalName, err := logicalNameProvider()
		if err != nil {
			// So that kube-aws can print a more useful error message with
			// the line number for the stack-template.json when there's an error
			return "", fmt.Errorf("failed to get arn: %v", err)
		}
		return fmt.Sprintf(`{ "Ref": %q }`, logicalName), nil
	})
}

func (i ARN) OrExpr(exprProvider func() (string, error)) (string, error) {
	if i.ArnFromStackOutput != "" {
		return fmt.Sprintf(`{ "Fn::ImportValue" : %q }`, i.ArnFromStackOutput), nil
	} else if i.Arn != "" {
		return fmt.Sprintf(`"%s"`, i.Arn), nil
	} else if i.ArnFromFn != "" {
		return i.ArnFromFn, nil
	} else {
		expr, err := exprProvider()
		if err != nil {
			// So that kube-aws can print a more useful error message with
			// the line number for the stack-template.json when there's an error
			return "", fmt.Errorf("failed to get arn: %v", err)
		}
		return expr, nil
	}
}
