package cmd

import (
	"fmt"
	"strconv"
	"strings"
)

type flag struct {
	name string
	val  string
}

func validateRequired(required ...flag) error {
	var missing []string
	for _, req := range required {
		if req.val == "" {
			missing = append(missing, strconv.Quote(req.name))
		}
	}
	if len(missing) != 0 {
		return fmt.Errorf("missing required flag(s): %s", strings.Join(missing, ", "))
	}
	return nil
}
