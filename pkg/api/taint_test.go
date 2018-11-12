package api

import (
	"testing"
)

func TestTaintString(t *testing.T) {
	taint := Taint{
		Key:    "key",
		Value:  "val",
		Effect: "NoSchedule",
	}

	expected := "key=val:NoSchedule"
	actual := taint.String()
	if actual != expected {
		t.Errorf("Expected taint string to be '%s', but was '%s", expected, actual)
	}
}

func TestTaintValidate(t *testing.T) {
	testCases := []struct {
		key     string
		effect  string
		isValid bool
	}{
		// Empty key
		{
			key:     "",
			effect:  "",
			isValid: false,
		},

		// Invalid effect
		{
			key:     "dedicated",
			effect:  "UnknownEffect",
			isValid: false,
		},

		// Valid taint
		{
			key:     "dedicated",
			effect:  "NoSchedule",
			isValid: true,
		},
	}

	for _, testCase := range testCases {
		taint := Taint{
			Key:    testCase.key,
			Value:  "",
			Effect: testCase.effect,
		}

		err := taint.Validate()

		if testCase.isValid && err != nil {
			t.Errorf("Expected taint to be valid, but got error: %v", err)

		}
		if !testCase.isValid && err == nil {
			t.Errorf("Expected taint to be invalid, but it was not")
		}
	}
}

func TestTaintsString(t *testing.T) {
	taints := Taints([]Taint{
		{
			Key:    "key-1",
			Value:  "val",
			Effect: "NoSchedule",
		},
		{
			Key:    "key-2",
			Value:  "val",
			Effect: "NoSchedule",
		},
	})

	expected := "key-1=val:NoSchedule,key-2=val:NoSchedule"
	actual := taints.String()
	if actual != expected {
		t.Errorf("Expected taints string to be '%s', but was '%s", expected, actual)
	}
}

func TestTaintsValidate(t *testing.T) {
	testCases := []struct {
		taints  Taints
		isValid bool
	}{
		// Unspecified key
		{
			taints: Taints{
				{
					Key:    "",
					Effect: "NoSchedule",
				},
			},
			isValid: false,
		},

		// Duplicate key/effect pair
		{
			taints: Taints{
				{
					Key:    "dedicated",
					Effect: "NoSchedule",
				},
				{
					Key:    "dedicated",
					Effect: "NoSchedule",
				},
			},
			isValid: false,
		},

		// Valid
		{
			taints: Taints{
				{
					Key:    "dedicated",
					Effect: "NoSchedule",
				},
				{
					Key:    "dedicated",
					Effect: "NoExecute",
				},
			},
			isValid: true,
		},
	}

	for _, testCase := range testCases {
		err := testCase.taints.Validate()

		if testCase.isValid && err != nil {
			t.Errorf("Expected taint to be valid, but got error: %v", err)
		}

		if !testCase.isValid && err == nil {
			t.Errorf("Expected taint to be invalid, but it was not")
		}
	}
}
