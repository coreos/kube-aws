package model

import (
	"strings"
)

// AdmissionControllers are modelled as a set of string names mapping to a positional integer
// the resultant list is ordered lowest -> highest positional value
type AdmissionControllers map[string]int

func (a AdmissionControllers) Enabled() bool {
	return len(a) > 0
}

// CommandString renders the admission controllers in the order defined by their integer positional values first
// and alphabetically should they have the same positional value
func (a AdmissionControllers) CommandString() string {
	var ordered []string
	var lowest int
	var nextkey string

	work := make(AdmissionControllers)
	for k, v := range a {
		work[k] = v
	}
	for n := 0; n < len(a); n++ {
		nextkey = ""
		for k, v := range work {
			if nextkey == "" {
				nextkey = k
				lowest = v
				continue
			}
			if v < lowest {
				lowest = v
				nextkey = k
				continue
			}
			if v == lowest {
				if strings.Compare(k, nextkey) == -1 {
					nextkey = k
					continue
				}
			}
		}
		ordered = append(ordered, nextkey)
		delete(work, nextkey)
	}
	return strings.Join(ordered, ",")
}
