package api

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	yaml "gopkg.in/yaml.v2"
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

// Convert the map[string]string FeatureGates to a map[string]bool yaml representation
func (l FeatureGates) Yaml() (string, error) {
	outmap := make(map[string]bool)
	var err error
	for k, v := range l {
		outmap[k], err = strconv.ParseBool(v)
		if err != nil {
			return "", err
		}
	}
	out, err := yaml.Marshal(&outmap)
	return string(out), err
}
