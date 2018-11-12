package api

import (
	"fmt"
	"strings"
)

// Taints is a list of taints
type Taints []Taint

// String returns a comma-separated list of taints
func (t Taints) String() string {
	ts := []string{}
	for _, t := range t {
		ts = append(ts, t.String())
	}
	return strings.Join(ts, ",")
}

// Validate returns an error if the list of taints are invalid as a group
func (t Taints) Validate() error {
	keyEffects := map[string]int{}

	for _, taint := range t {
		if err := taint.Validate(); err != nil {
			return err
		}

		keyEffect := taint.Key + ":" + taint.Effect
		if _, ok := keyEffects[keyEffect]; ok {
			return fmt.Errorf("taints must be unique by key and effect pair")
		} else {
			keyEffects[keyEffect] = 1
		}
	}

	return nil
}

// Taint is a k8s node taint which is added to nodes which requires pods to tolerate
type Taint struct {
	Key    string `yaml:"key"`
	Value  string `yaml:"value"`
	Effect string `yaml:"effect"`
}

// String returns a taint represented in string
func (t Taint) String() string {
	return fmt.Sprintf("%s=%s:%s", t.Key, t.Value, t.Effect)
}

// Validate returns an error if the taint is invalid
func (t Taint) Validate() error {
	if len(t.Key) == 0 {
		return fmt.Errorf("expected taint key to be a non-empty string")
	}

	if t.Effect != "NoSchedule" && t.Effect != "PreferNoSchedule" && t.Effect != "NoExecute" {
		return fmt.Errorf("invalid taint effect: %s", t.Effect)
	}

	return nil
}
