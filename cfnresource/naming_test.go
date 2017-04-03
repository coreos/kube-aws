package cfnresource

import "testing"

func TestValidateRoleNameLength(t *testing.T) {
	t.Run("WhenMax", func(t *testing.T) {
		if e := ValidateRoleNameLength("my-firstcluster", "prodWorkerks", "prod-workers", "us-east-1"); e != nil {
			t.Errorf("expected validation to succeed but failed: %v", e)
		}
	})
	t.Run("WhenTooLong", func(t *testing.T) {
		if e := ValidateRoleNameLength("my-secondcluster", "prodWorkerks", "prod-workers", "us-east-1"); e == nil {
			t.Error("expected validation to fail but succeeded")
		}
	})
}
