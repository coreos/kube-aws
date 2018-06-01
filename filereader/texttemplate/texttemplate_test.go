package texttemplate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var tolabelFunc = funcs2["toLabel"].(func(string) string)

func TestCIDRToLabel(t *testing.T) {

	data := "192.168.0.0/16"
	label := tolabelFunc(data)

	assert.Equal(t, "192.168.0.0_16", label)
}

func TestMultipleSymbolsToReplace(t *testing.T) {

	data := "https://kubernetes.io/docs/admin/authorization/rbac/#referring-to-subjects"
	label := tolabelFunc(data)

	assert.Equal(t, "https___kubernetes.io_docs_admin_authorization_rbac__referring-to-subjects", label)
}
