package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func mustHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}
	return string(hash)
}

func TestCheckBasicAuth(t *testing.T) {
	hash := mustHash(t, "secret")
	cred := ServiceCred{
		HostGlob:     "test.example.com",
		Username:     "tester",
		PasswordHash: hash,
	}

	if !CheckBasicAuth(cred, "tester", "secret") {
		t.Fatal("expected valid credentials to pass")
	}
	if CheckBasicAuth(cred, "tester", "wrong") {
		t.Fatal("expected wrong password to fail")
	}
	if CheckBasicAuth(cred, "other", "secret") {
		t.Fatal("expected wrong username to fail")
	}
	if CheckBasicAuth(cred, "", "secret") {
		t.Fatal("expected empty username to fail")
	}
	if CheckBasicAuth(cred, "tester", "") {
		t.Fatal("expected empty password to fail")
	}
}

func TestCheckBasicAuthAgainstServices(t *testing.T) {
	creds := []ServiceCred{
		{
			HostGlob:     "a.example.com",
			Username:     "alice",
			PasswordHash: mustHash(t, "alice-pass"),
		},
		{
			HostGlob:     "b.example.com",
			Username:     "bob",
			PasswordHash: mustHash(t, "bob-pass"),
		},
	}

	got, ok := CheckBasicAuthAgainstServices(creds, "bob", "bob-pass")
	if !ok || got.Username != "bob" {
		t.Fatalf("expected bob match, got %#v ok=%v", got, ok)
	}

	_, ok = CheckBasicAuthAgainstServices(creds, "bob", "nope")
	if ok {
		t.Fatal("expected mismatch")
	}
}

func TestCheckBasicAuthHashWithSlashes(t *testing.T) {
	hash := mustHash(t, "slashy")
	cred := ServiceCred{Username: "u", PasswordHash: hash}
	if !CheckBasicAuth(cred, "u", "slashy") {
		t.Fatal("expected hash with possible '/' to verify")
	}
}
