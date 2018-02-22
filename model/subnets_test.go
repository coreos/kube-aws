package model

import (
	"testing"
)

func TestSubnetsMixed(t *testing.T) {

	public := Subnet{Name: "Public", AvailabilityZone: "ap-northeast-1a", InstanceCIDR: "10.0.0.0/24", Private: false}
	s1 := Subnets{
		public,
	}
	if s1.ContainsBothPrivateAndPublic() {
		t.Error("Func ContainsBothPrivateAndPublic should return false when there is only one public subnet in Subnets but it did not")
	}

	private := Subnet{Name: "Private", AvailabilityZone: "ap-northeast-1b", InstanceCIDR: "10.0.1.0/24", Private: true}
	s2 := Subnets{
		private,
	}
	if s2.ContainsBothPrivateAndPublic() {
		t.Error("Func ContainsBothPrivateAndPublic should return false when there is only one private subnet in Subnets but it did not")
	}

	s3 := Subnets{
		public,
		private,
	}
	if !s3.ContainsBothPrivateAndPublic() {
		t.Error("Func ContainsBothPrivateAndPublic should return true when the set of subnets contains both private and public subnet(s) but it did not")
	}
}
