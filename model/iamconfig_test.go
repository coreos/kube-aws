package model

import "testing"

func testValidIAMProfile(arn string, t *testing.T) {
	c := IAMConfig{
		InstanceProfile: IAMInstanceProfile{
			ARN: ARN{Arn: arn},
		},
	}
	if err := c.Validate(); err != nil {
		t.Errorf("iam profile should have been valid but raised error")
	}
}

func TestGovCloudIAMProfile(t *testing.T) {
	testValidIAMProfile("arn:aws-us-gov:iam::123412341234:instance-profile/Test", t)
}

func TestChinaIAMProfile(t *testing.T) {
	testValidIAMProfile("arn:aws-cn:iam::123412341234:instance-profile/Test", t)
}

func TestIAMProfile(t *testing.T) {
	testValidIAMProfile("arn:aws:iam::123412341234:instance-profile/Test", t)
}
