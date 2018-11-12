package api

import (
	"fmt"
	"sort"
	"strings"
)

type FeatureGates map[string]string

func (l FeatureGates) Enabled() bool {
	return len(l) > 0
}

// Returns key=value pairs separated by ',' to be passed to kubelet's `--feature-gates` flag
func (l FeatureGates) String() string {
	labels := []string{}
	keys := []string{}
	for k, _ := range l {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := l[k]
		if len(v) > 0 {
			labels = append(labels, fmt.Sprintf("%s=%s", k, v))
		} else {
			labels = append(labels, fmt.Sprintf("%s", k))
		}
	}
	return strings.Join(labels, ",")
}
