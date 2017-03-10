package model

import (
	"fmt"
)

type Region interface {
	PrivateDomainName() string
	PublicDomainName() string
	String() string
}

type regionImpl struct {
	name string
}

func RegionForName(name string) Region {
	return regionImpl{
		name: name,
	}
}

func (r regionImpl) PrivateDomainName() string {
	if r.name == "us-east-1" {
		return "ec2.internal"
	}
	return fmt.Sprintf("%s.compute.internal", r.name)
}

func (r regionImpl) PublicDomainName() string {
	switch r.name {
	case "cn-north-1":
		return fmt.Sprintf("%s.compute.amazonaws.com.cn", r.name)
	case "us-east-1":
		return "compute-1.amazonaws.com"
	default:
		return fmt.Sprintf("%s.compute.amazonaws.com", r.name)
	}
}

func (r regionImpl) String() string {
	return r.name
}
