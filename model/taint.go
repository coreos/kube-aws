package model

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
