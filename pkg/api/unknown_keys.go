package api

import (
	"fmt"
	"sort"
	"strings"
)

type UnknownKeys map[string]interface{}

func (unknownKeys UnknownKeys) FailWhenUnknownKeysFound(keyPath string) error {
	if unknownKeys != nil && len(unknownKeys) > 0 {
		ks := []string{}
		for k, _ := range unknownKeys {
			ks = append(ks, k)
		}

		sort.Strings(ks)

		if keyPath != "" {
			return fmt.Errorf("unknown keys found in %s: %s", keyPath, strings.Join(ks, ", "))
		}
		return fmt.Errorf("unknown keys found: %s", strings.Join(ks, ", "))
	}
	return nil
}
