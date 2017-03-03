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
	if r.name == "cn-north-1" {
		return fmt.Sprintf("%s.compute.amazonaws.com.cn", r.name)
	}
	return fmt.Sprintf("%s.compute.amazonaws.com", r.name)
}

func (r regionImpl) String() string {
	return r.name
}
