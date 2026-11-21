package config

import "testing"

func TestListenPort(t *testing.T) {
	t.Run("PORT wins over APP_PORT", func(t *testing.T) {
		t.Setenv("PORT", "3000")
		t.Setenv("APP_PORT", "8080")
		if got := listenPort(); got != "3000" {
			t.Fatalf("listenPort() = %q, want 3000", got)
		}
	})

	t.Run("APP_PORT when PORT is unset", func(t *testing.T) {
		t.Setenv("PORT", "")
		t.Setenv("APP_PORT", "8080")
		if got := listenPort(); got != "8080" {
			t.Fatalf("listenPort() = %q, want 8080", got)
		}
	})

	t.Run("default 8080", func(t *testing.T) {
		t.Setenv("PORT", "")
		t.Setenv("APP_PORT", "")
		if got := listenPort(); got != "8080" {
			t.Fatalf("listenPort() = %q, want 8080", got)
		}
	})
}
