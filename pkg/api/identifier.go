package api

import (
	"encoding/json"
	"fmt"
)

type Identifier struct {
	ID                string `yaml:"id,omitempty"`
	IDFromStackOutput string `yaml:"idFromStackOutput,omitempty"`
	IDFromFn          string `yaml:"idFromFn,omitempty"`
}

// HasIdentifier returns true when the id of a resource i.e. either `id` or `idFromStackOutput` is specified
func (i Identifier) HasIdentifier() bool {
	return i.ID != "" || i.IDFromStackOutput != ""
}

func (i Identifier) Validate() error {
	if i.IDFromFn != "" {
		var jsonHolder map[string]interface{}
		if err := json.Unmarshal([]byte(i.IDFromFn), &jsonHolder); err != nil {
			return fmt.Errorf("idFromFn must be a valid json expression but was not: %s", i.IDFromFn)
		}
	}
	return nil
}

func (i Identifier) Ref(logicalNameProvider func() string) string {
	if i.IDFromStackOutput != "" {
		return fmt.Sprintf(`{ "Fn::ImportValue" : %q }`, i.IDFromStackOutput)
	} else if i.ID != "" {
		return fmt.Sprintf(`"%s"`, i.ID)
	} else if i.IDFromFn != "" {
		return i.IDFromFn
	} else {
		return fmt.Sprintf(`{ "Ref" : %q }`, logicalNameProvider())
	}
}

// RefOrError should be used instead of Ref where possible so that kube-aws can print a more useful error message with
// the line number for the stack-template.json when there's an error.
func (i Identifier) RefOrError(logicalNameProvider func() (string, error)) (string, error) {
	if i.IDFromStackOutput != "" {
		return fmt.Sprintf(`{ "Fn::ImportValue" : %q }`, i.IDFromStackOutput), nil
	} else if i.ID != "" {
		return fmt.Sprintf(`"%s"`, i.ID), nil
	} else if i.IDFromFn != "" {
		return i.IDFromFn, nil
	} else {
		logicalName, err := logicalNameProvider()
		if err != nil {
			return "", fmt.Errorf("failed to get id or ref: %v", err)
		}
		return fmt.Sprintf(`{ "Ref" : %q }`, logicalName), nil
	}
}
