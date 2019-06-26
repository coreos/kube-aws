package pluginutil

import (
	"fmt"

	"github.com/imdario/mergo"
)

func MergeValues(v, o map[string]interface{}) (map[string]interface{}, error) {
	work := make(map[string]interface{})
	err := mergo.Merge(&work, o)
	if err != nil {
		return work, fmt.Errorf("Could not merge source plugin values into work variable")
	}
	err = mergo.Merge(&work, v)
	if err != nil {
		return work, fmt.Errorf("Could not merge target values into plugin source values")
	}
	return work, nil
}
