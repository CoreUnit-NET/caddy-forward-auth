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
