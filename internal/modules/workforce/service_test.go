package workforce

import "testing"

func TestStaffRoleValidation(t *testing.T) {
	if validRole("admin") {
		t.Fatal("unsupported staff role must be rejected")
	}
	if !validRole("manager") {
		t.Fatal("manager role should be accepted")
	}
}
