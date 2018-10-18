package model

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"
)

type FeatureGates map[string]bool

func (l FeatureGates) Enabled() bool {
	return len(l) > 0
}

// Returns key=value pairs separated by ',' to be passed to kubelet's `--feature-gates` flag
func (l FeatureGates) CommandString() string {
	g := []string{}
	for k, v := range l {
		g = append(g, fmt.Sprintf("%s=%v", k, v))
	}
	sort.Strings(g)
	return strings.Join(g, ",")
}

func (l FeatureGates) ToYaml() string {
	y, err := yaml.Marshal(l)
	if err != nil {
		return ""
	}
	return string(y)
}
