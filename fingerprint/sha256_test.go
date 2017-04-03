package fingerprint

import (
	"testing"
)

func TestSHA256(t *testing.T) {
	actual := SHA256("mychangingdata")
	expected := "ee867acc5d96cced9b9fe075e293604214519650065c60b42b95f1ccfbac2c97"
	if actual != expected {
		t.Errorf("unexpected value returned from SHA256: expected=%v actual=%v", expected, actual)
	}
}
