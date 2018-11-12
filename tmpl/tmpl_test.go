package tmpl

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestTextToCfnExprTokens(t *testing.T) {
	testCases := []struct {
		expected []json.RawMessage
		src      string
	}{
		{
			expected: []json.RawMessage{json.RawMessage(`"foobar"`)},
			src:      `foobar`,
		},
		{
			expected: []json.RawMessage{json.RawMessage(`"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: aws-auth\ndata:\n  mapRoles: |\n    - rolearn: "`),
				json.RawMessage(`{"Fn::GetAtt": ["IAMRoleController", "Arn"]}`),
			},
			src: `apiVersion: v1
kind: ConfigMap
metadata:
  name: aws-auth
data:
  mapRoles: |
    - rolearn: {"Fn::GetAtt": ["IAMRoleController", "Arn"]}`,
		},
	}

	for _, testCase := range testCases {
		actual := TextToCfnExprTokens(testCase.src)
		if !reflect.DeepEqual(testCase.expected, actual) {
			t.Errorf("unexpected result: expected=%+v, actual=%+v", testCase.expected, actual)
		}
	}
}
