package cfnresource

import "testing"

func TestValidateRoleNameLength(t *testing.T) {
	t.Run("WhenMax", func(t *testing.T) {
		if e := ValidateUnstableRoleNameLength("my-firstcluster", "prodWorkerks", "prod-workers", "us-east-1"); e != nil {
			t.Errorf("expected validation to succeed but failed: %v", e)
		}
	})
	t.Run("WhenTooLong", func(t *testing.T) {
		if e := ValidateUnstableRoleNameLength("my-secondcluster", "prodWorkerks", "prod-workers", "us-east-1"); e == nil {
			t.Error("expected validation to fail but succeeded")
		}
	})
}

func TestValidateManagedRoleNameLength(t *testing.T) {
	t.Run("WhenMax", func(t *testing.T) {
		if e := ValidateStableRoleNameLength("prod-workers", "ap-southeast-1"); e != nil {
			t.Errorf("expected validation to succeed but failed: %v", e)
		}
	})
	t.Run("WhenTooLong", func(t *testing.T) {
		if e := ValidateStableRoleNameLength("prod-workers-role-with-very-very-very-very-very-long-name", "ap-southeast-1"); e == nil {
			t.Error("expected validation to fail but succeeded")
		}
	})
}
