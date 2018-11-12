package pluginutil

import (
	"fmt"

	"github.com/kubernetes-incubator/kube-aws/pkg/api"
)

func MergeValues(v api.Values, o map[string]interface{}) api.Values {
	r := merge(map[string]interface{}(v), map[string]interface{}(o))
	switch r := r.(type) {
	case map[string]interface{}:
		return api.Values(r)
	}
	panic(fmt.Errorf("error in type assertion to map[string]interface{} from merge result: %v", r))
}

func merge(x1, x2 interface{}) interface{} {
	switch x1 := x1.(type) {
	case map[string]interface{}:
		x2, ok := x2.(map[string]interface{})
		if !ok {
			panic(fmt.Sprintf("cannot merge map[string]interface{} %+v and %+v", x1, x2))
		}
		for k, v2 := range x2 {
			if v1, ok := x1[k]; ok {
				x1[k] = merge(v1, v2)
			} else {
				x1[k] = v2
			}
		}
		return x1
	case map[string]string:
		x2, ok := x2.(map[string]string)
		if !ok {
			panic(fmt.Sprintf("cannot merge map[string]string %+v and %+v", x1, x2))
		}
		for k, v2 := range x2 {
			x1[k] = v2
		}
		r := map[string]interface{}{}
		for k, v := range x1 {
			r[k] = string(v)
		}
		return r
	case nil:
		panic(fmt.Sprintf("cannot merge nil and %+v", x2))
	}
	return x2
}
