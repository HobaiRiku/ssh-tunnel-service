package paths

import (
	"path/filepath"
	"testing"
)

func TestResolvePrecedence(t *testing.T) {
	// Explicit override wins over everything, including the env var.
	t.Setenv(envHome, "/from/env")
	p, err := Resolve("/explicit/home")
	if err != nil {
		t.Fatalf("Resolve override: %v", err)
	}
	if p.Home != "/explicit/home" {
		t.Errorf("override: Home = %q, want /explicit/home", p.Home)
	}

	// With no override, the env var is used.
	p, err = Resolve("")
	if err != nil {
		t.Fatalf("Resolve env: %v", err)
	}
	if p.Home != "/from/env" {
		t.Errorf("env: Home = %q, want /from/env", p.Home)
	}
}

// TestUserHome covers the per-user default independently of the process's
// privilege level (which selects between userHome and the platform system path).
func TestUserHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := userHome()
	if err != nil {
		t.Fatalf("userHome: %v", err)
	}
	want := filepath.Join(home, defaultDirName)
	if got != want {
		t.Errorf("userHome = %q, want %q", got, want)
	}
}
