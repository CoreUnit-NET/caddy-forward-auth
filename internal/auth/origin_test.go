package auth

import "testing"

func TestOriginAllowed(t *testing.T) {
	allowed := []string{"intern-auth.example.com", "localhost", "auth-test.example.com"}

	tests := []struct {
		name   string
		origin string
		list   []string
		want   bool
	}{
		{"no list", "https://evil.example.com", nil, true},
		{"empty origin", "", allowed, true},
		{"allowed https", "https://intern-auth.example.com", allowed, true},
		{"allowed with port", "http://localhost:3000", allowed, true},
		{"case insensitive", "https://Intern-Auth.Example.COM", allowed, true},
		{"blocked", "https://evil.example.com", allowed, false},
		{"bare hostname allowed", "localhost", allowed, true},
		{"invalid origin", "://", allowed, false},
		{"allowed list with port", "https://localhost", []string{"localhost:3000"}, true},
		{"allowed list as url", "https://intern-auth.example.com", []string{"https://Intern-Auth.Example.COM"}, true},
		{"allowed list url with port", "http://auth-test.example.com:8080", []string{"https://auth-test.example.com:443"}, true},
		{"glob single label", "https://foo.intern.example.com", []string{"*.intern.example.com"}, true},
		{"glob multi label blocked", "https://a.b.intern.example.com", []string{"*.intern.example.com"}, false},
		{"glob bare star", "https://anything.example.com", []string{"*"}, true},
		{"glob mixed with exact", "https://ai-dashboard.intern.coreunit.net", []string{"gateway.example.com", "*.intern.coreunit.net"}, true},
		{"glob as url pattern", "https://bar.intern.example.com", []string{"https://*.intern.example.com"}, true},
		{"glob no match", "https://evil.example.com", []string{"*.intern.example.com"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OriginAllowed(tt.origin, tt.list)
			if got != tt.want {
				t.Fatalf("OriginAllowed(%q) = %v, want %v", tt.origin, got, tt.want)
			}
		})
	}
}

func TestOriginHostname(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"  ", ""},
		{"Example.COM", "example.com"},
		{"example.com:443", "example.com"},
		{"https://Example.COM:8443/path", "example.com"},
		{"http://localhost:3000", "localhost"},
		{"://", ""},
		{"*.intern.example.com", "*.intern.example.com"},
		{"https://*.intern.example.com", "*.intern.example.com"},
		{"*", "*"},
	}
	for _, tt := range tests {
		got, ok := originHostname(tt.in)
		if !ok {
			got = ""
		}
		if got != tt.want {
			t.Fatalf("originHostname(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
