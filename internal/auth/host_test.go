package auth

import (
	"testing"
)

func TestHostMatches(t *testing.T) {
	tests := []struct {
		glob string
		host string
		want bool
	}{
		{"test.example.com", "test.example.com", true},
		{"test.example.com", "TEST.EXAMPLE.COM", true},
		{"test.example.com", "other.example.com", false},
		{"*.intern.example.com", "foo.intern.example.com", true},
		{"*.intern.example.com", "a.b.intern.example.com", false},
		{"*.intern.example.com", "intern.example.com", false},
		{"*", "anything.example.com", true},
		{"", "test.example.com", false},
		{"test.example.com", "", false},
		{"test.example.com", "test.example.com:8443", true},
	}
	for _, tt := range tests {
		got := HostMatches(tt.glob, tt.host)
		if got != tt.want {
			t.Fatalf("HostMatches(%q, %q) = %v, want %v", tt.glob, tt.host, got, tt.want)
		}
	}
}

func TestFindServicesForHost(t *testing.T) {
	services := map[string]ServiceCred{
		"test": {
			HostGlob:     "test.example.com",
			Username:     "tester",
			PasswordHash: "hash-test",
		},
		"intern": {
			HostGlob:     "*.intern.example.com",
			Username:     "intern-user",
			PasswordHash: "hash-intern",
		},
		"other": {
			HostGlob:     "other.example.com",
			Username:     "other",
			PasswordHash: "hash-other",
		},
	}

	got := FindServicesForHost(services, "api.intern.example.com")
	if len(got) != 1 || got[0].Username != "intern-user" {
		t.Fatalf("unexpected matches: %#v", got)
	}

	got = FindServicesForHost(services, "test.example.com:8080")
	if len(got) != 1 || got[0].Username != "tester" {
		t.Fatalf("unexpected exact matches: %#v", got)
	}

	got = FindServicesForHost(services, "unknown.example.com")
	if len(got) != 0 {
		t.Fatalf("expected no matches, got %#v", got)
	}
}
