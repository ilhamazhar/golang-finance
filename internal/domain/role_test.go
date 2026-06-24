package domain

import "testing"

func TestCanViewAllResources(t *testing.T) {
	cases := []struct {
		role Role
		want bool
	}{
		{RoleAdmin, true},
		{RoleStaff, true},
		{RoleUser, false},
		{Role("unknown"), false},
		{Role(""), false},
	}
	for _, tc := range cases {
		if got := CanViewAllResources(tc.role); got != tc.want {
			t.Errorf("CanViewAllResources(%q) = %v, want %v", tc.role, got, tc.want)
		}
	}
}
