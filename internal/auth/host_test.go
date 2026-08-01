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

func TestHostGlobsOverlap(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"*.example.com", "api.example.com", true},
		{"*.example.com", "*.example.com", true},
		{"*", "anything.example.com", true},
		{"a.example.com", "b.example.com", false},
		{"*.example.com", "*.other.com", false},
		{"*.intern.example.com", "api.intern.example.com", true},
		{"*.example.com", "*.*.example.com", false},
		{"", "api.example.com", false},
	}
	for _, tt := range tests {
		got := HostGlobsOverlap(tt.a, tt.b)
		if got != tt.want {
			t.Fatalf("HostGlobsOverlap(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
		got = HostGlobsOverlap(tt.b, tt.a)
		if got != tt.want {
			t.Fatalf("HostGlobsOverlap(%q, %q) = %v, want %v", tt.b, tt.a, got, tt.want)
		}
	}
}

func TestOverlappingHostGlobPairs(t *testing.T) {
	services := map[string]ServiceCred{
		"exact":  {HostGlob: "api.example.com", Username: "a", PasswordHash: "h1"},
		"glob":   {HostGlob: "*.example.com", Username: "b", PasswordHash: "h2"},
		"unique": {HostGlob: "unique.test", Username: "d", PasswordHash: "h4"},
	}

	pairs := OverlappingHostGlobPairs(services)
	if len(pairs) != 1 {
		t.Fatalf("pairs = %#v, want one overlapping pair", pairs)
	}
	if pairs[0][0] != "exact" || pairs[0][1] != "glob" {
		t.Fatalf("pair = %#v, want [exact glob]", pairs[0])
	}
}
