package model

import (
	"fmt"
)

type Identifier struct {
	ID                string `yaml:"id,omitempty"`
	IDFromStackOutput string `yaml:"idFromStackOutput,omitempty"`
}

func (c Identifier) HasIdentifier() bool {
	return c.ID != "" || c.IDFromStackOutput != ""
}

func (c Identifier) Ref(logicalName string) string {
	if c.IDFromStackOutput != "" {
		return fmt.Sprintf(`{ "ImportValue" : %q }`, c.IDFromStackOutput)
	} else if c.ID != "" {
		return fmt.Sprintf(`{ "Ref" : %q }`, c.ID)
	} else {
		return fmt.Sprintf(`{ "Ref" : %q }`, logicalName)
	}
}
