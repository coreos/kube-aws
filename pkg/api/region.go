package api

import (
	"fmt"
	"strings"
)

type Region struct {
	Name string `yaml:"region,omitempty"`
}

func RegionForName(name string) Region {
	return Region{
		Name: name,
	}
}

func (r Region) PrivateDomainName() string {
	if r.Name == "us-east-1" {
		return "ec2.internal"
	}
	return fmt.Sprintf("%s.compute.internal", r.Name)
}

func (r Region) PublicComputeDomainName() string {
	switch r.Name {
	case "us-east-1":
		return fmt.Sprintf("compute-1.%s", r.PublicDomainName())
	default:
		return fmt.Sprintf("%s.compute.%s", r.Name, r.PublicDomainName())
	}
}

func (r Region) PublicDomainName() string {
	if r.IsChina() {
		return "amazonaws.com.cn"
	}
	return "amazonaws.com"
}

func (r Region) String() string {
	return r.Name
}

func (r Region) S3Endpoint() string {
	if r.IsChina() {
		return fmt.Sprintf("https://s3.%s.amazonaws.com.cn", r.Name)
	}
	if r.IsGovcloud() {
		return fmt.Sprintf("https://s3-%s.amazonaws.com", r.Name)
	}
	return "https://s3.amazonaws.com"
}

func (r Region) Partition() string {
	if r.IsChina() {
		return "aws-cn"
	}
	if r.IsGovcloud() {
		return "aws-us-gov"
	}
	return "aws"
}

func (r Region) IsChina() bool {
	return strings.HasPrefix(r.Name, "cn-")
}

func (r Region) IsGovcloud() bool {
	return strings.HasPrefix(r.Name, "us-gov-")
}

func (r Region) IsEmpty() bool {
	return r.Name == ""
}

func (r Region) SupportsKMS() bool {
	return !r.IsChina()
}

func (r Region) SupportsNetworkLoadBalancers() bool {
	return !r.IsChina()
}
