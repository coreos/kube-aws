package api

import (
	"fmt"
	"github.com/go-yaml/yaml"
	"testing"
)

func TestCIDRRangesExtractFromYAML(t *testing.T) {
	t.Run("WhenOverrodeWithNonEmpty", func(t *testing.T) {
		rs := struct {
			CIDRRanges `yaml:"rs"`
		}{DefaultCIDRRanges()}
		err := yaml.Unmarshal([]byte("rs:\n- \"1.2.3.255/32\"\n"), &rs)
		if err != nil {
			t.Errorf("failed ot extract CIDR ranges from yaml: %v", err)
			t.FailNow()
		}
		expected := "1.2.3.255/32"
		actual := rs.CIDRRanges[0].str
		if actual != expected {
			t.Errorf("unexpected cidr range extracted. expected = %s, actual = %s", expected, actual)
		}
	})
	t.Run("WhenOverrodeWithEmpty", func(t *testing.T) {
		rs := struct {
			CIDRRanges `yaml:"rs"`
		}{DefaultCIDRRanges()}
		err := yaml.Unmarshal([]byte("rs:\n"), &rs)
		if err != nil {
			t.Errorf("failed ot extract CIDR ranges from yaml: %v", err)
			t.FailNow()
		}
		if len(rs.CIDRRanges) != 0 {
			t.Errorf("unexpected cidr ranges to be empty, but was: %+v(len=%d)", rs.CIDRRanges, len(rs.CIDRRanges))
		}
	})
	t.Run("WhenOmitted", func(t *testing.T) {
		rs := struct {
			CIDRRanges `yaml:"rs"`
		}{DefaultCIDRRanges()}
		err := yaml.Unmarshal([]byte(""), &rs)
		if err != nil {
			t.Errorf("failed ot extract CIDR ranges from yaml: %v", err)
			t.FailNow()
		}
		expected := "0.0.0.0/0"
		actual := rs.CIDRRanges[0].str
		if actual != expected {
			t.Errorf("unexpected cidr range extracted. expected = %s, actual = %s", expected, actual)
		}
	})
}

func TestCIDRRangeExtractFromYAML(t *testing.T) {
	r := CIDRRange{}
	err := yaml.Unmarshal([]byte("\"0.0.0.0/0\""), &r)
	if err != nil {
		t.Errorf("failed to extract CIDR range from yaml: %v", err)
	}
	expected := "0.0.0.0/0"
	if r.str != expected {
		t.Errorf("unexpected cidr range extracted. expected = %s, actual = %s", expected, r.str)
	}
}

func TestCIDRRangeString(t *testing.T) {
	r := CIDRRange{str: "0.0.0.0/0"}
	expected := "0.0.0.0/0"
	actual := fmt.Sprintf("%s", r)
	if actual != expected {
		t.Errorf("unexpected string rendered. expected = %s, actual = %s", expected, actual)
	}
}
