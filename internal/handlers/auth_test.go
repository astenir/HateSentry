package handlers

import "testing"

func TestRegistrationRoleForUserCount(t *testing.T) {
	tests := []struct {
		name      string
		userCount int64
		wantRole  string
	}{
		{
			name:      "first registered user bootstraps admin",
			userCount: 0,
			wantRole:  "admin",
		},
		{
			name:      "later registered users are normal users",
			userCount: 1,
			wantRole:  "user",
		},
		{
			name:      "many existing users remain normal registrations",
			userCount: 42,
			wantRole:  "user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := registrationRoleForUserCount(tt.userCount); got != tt.wantRole {
				t.Fatalf("registrationRoleForUserCount(%d) = %q, want %q", tt.userCount, got, tt.wantRole)
			}
		})
	}
}
