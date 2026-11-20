package user

import "testing"

func TestRoleValid(t *testing.T) {
	cases := map[Role]bool{
		RoleStudent: true,
		RoleTeacher: true,
		RoleAdmin:   true,
		"":          false,
		"superuser": false,
		"Admin":     false,
	}
	for role, want := range cases {
		if got := role.Valid(); got != want {
			t.Fatalf("Role(%q).Valid() = %v, want %v", role, got, want)
		}
	}
}
