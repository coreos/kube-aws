package naming

import (
	"strings"
)

func FromStackToCfnResource(stackName string) string {
	// Convert stack name into something valid as a cfn resource name or
	// we'll end up with cfn errors like "Template format error: Resource name test5-controlplane is non alphanumeric"
	return strings.Title(strings.Replace(stackName, "-", "", -1))
}
