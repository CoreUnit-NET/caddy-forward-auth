package config

import "testing"

func TestEnvIsString(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		t.Setenv("TEST_STRING", "hello")
		var got string
		if err := envIsString("TEST_STRING", func(value string) {
			got = value
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "hello" {
			t.Fatalf("got %q, want hello", got)
		}
	})

	t.Run("empty", func(t *testing.T) {
		t.Setenv("TEST_STRING", "")
		called := false
		if err := envIsString("TEST_STRING", func(string) { called = true }); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if called {
			t.Fatal("expected callback to be skipped for empty env")
		}
	})
}

func TestEnvIsInt(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		t.Setenv("TEST_INT", "42")
		var got int
		if err := envIsInt("TEST_INT", func(value int) { got = value }); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 42 {
			t.Fatalf("got %d, want 42", got)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Setenv("TEST_INT", "nope")
		called := false
		err := envIsInt("TEST_INT", func(int) { called = true })
		if err == nil {
			t.Fatal("expected error for invalid int")
		}
		if called {
			t.Fatal("expected callback to be skipped for invalid int")
		}
	})

	t.Run("empty", func(t *testing.T) {
		t.Setenv("TEST_INT", "")
		called := false
		if err := envIsInt("TEST_INT", func(int) { called = true }); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if called {
			t.Fatal("expected callback to be skipped for empty env")
		}
	})
}

func TestEnvIsBool(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		t.Setenv("TEST_BOOL", "true")
		var got bool
		if err := envIsBool("TEST_BOOL", func(value bool) { got = value }); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got {
			t.Fatal("expected true")
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Setenv("TEST_BOOL", "maybe")
		called := false
		err := envIsBool("TEST_BOOL", func(bool) { called = true })
		if err == nil {
			t.Fatal("expected error for invalid bool")
		}
		if called {
			t.Fatal("expected callback to be skipped for invalid bool")
		}
	})

	t.Run("empty", func(t *testing.T) {
		t.Setenv("TEST_BOOL", "")
		called := false
		if err := envIsBool("TEST_BOOL", func(bool) { called = true }); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if called {
			t.Fatal("expected callback to be skipped for empty env")
		}
	})
}
