package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-yaml/yaml"
)

type fakeConfig struct {
	Name        string `yaml:"name,omitempty"`
	UnknownKeys `yaml:",inline"`
}

func TestUnknownKeys(t *testing.T) {
	t.Run("WithoutKeyPath", func(t *testing.T) {
		data := `name: myname
unknownKey1: unusedValue1
unknownKey2: unusedValue2
`
		c := fakeConfig{}
		yamlErr := yaml.Unmarshal([]byte(data), &c)
		if yamlErr != nil {
			t.Errorf("bug in test! %v", yamlErr)
			t.FailNow()
		}
		e := c.FailWhenUnknownKeysFound("")
		if e == nil {
			t.Error("expected to fail but succeeded")
		}
		m := fmt.Sprintf("%v", e)
		if !strings.Contains(m, `unknown keys found: unknownKey1, unknownKey2`) {
			t.Errorf("unexpected error message from FailWhenUnknownKeysFound(): %v", m)
		}
	})

	t.Run("WithKeyPath", func(t *testing.T) {
		data := `name: myname
unknownKey1: unusedValue1
unknownKey2: unusedValue2
`
		c := fakeConfig{}
		yamlErr := yaml.Unmarshal([]byte(data), &c)
		if yamlErr != nil {
			t.Errorf("bug in test! %v", yamlErr)
			t.FailNow()
		}
		e := c.FailWhenUnknownKeysFound("worker.nodePools[0]")
		if e == nil {
			t.Error("expected to fail but succeeded")
		}
		m := fmt.Sprintf("%v", e)
		if !strings.Contains(m, `unknown keys found in worker.nodePools[0]: unknownKey1, unknownKey2`) {
			t.Errorf("unexpected error message from FailWhenUnknownKeysFound(): %v", m)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		data := `name: myname
`
		c := fakeConfig{}
		yamlErr := yaml.Unmarshal([]byte(data), &c)
		if yamlErr != nil {
			t.Errorf("bug in test! %v", yamlErr)
			t.FailNow()
		}
		e := c.FailWhenUnknownKeysFound("")
		if e != nil {
			t.Errorf("expected to succeed but failed: %v", e)
		}
	})
}
