package api

import (
	"fmt"
	"net"
)

// CIDRRanges represents IP network ranges in CIDR notation
type CIDRRanges []CIDRRange

// CIDRRange represents an IP network range in CIDR notation
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-security-group-ingress.html#cfn-ec2-security-group-ingress-cidrip
type CIDRRange struct {
	str string
}

func DefaultCIDRRanges() CIDRRanges {
	return CIDRRanges{
		{"0.0.0.0/0"},
	}
}

func (c *CIDRRange) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var cidr string
	if err := unmarshal(&cidr); err != nil {
		return fmt.Errorf("failed to parse CIDR range: %v", err)
	}

	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("failed to parse CIDR range: %v", err)
	}

	*c = CIDRRange{str: cidr}

	return nil
}

// String returns the string representation of this CIDR range
func (c CIDRRange) String() string {
	return c.str
}
