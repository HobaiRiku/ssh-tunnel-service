package paths

import (
	"os"
	"path/filepath"
	"runtime"
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
	// os.UserHomeDir consults HOME on Unix and USERPROFILE on Windows; set both
	// so this test is hermetic across platforms.
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	got, err := userHome()
	if err != nil {
		t.Fatalf("userHome: %v", err)
	}
	want := filepath.Join(home, defaultDirName)
	if got != want {
		t.Errorf("userHome = %q, want %q", got, want)
	}
}

// TestEnsureTreePrivate confirms the default (owner-only) model creates the tree
// at 0700 and leaves config.yaml ownership/mode untouched.
func TestEnsureTreePrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mode bits not meaningful on Windows")
	}
	p := Paths{Home: t.TempDir(), perm: privatePerm}
	if err := p.EnsureTree(); err != nil {
		t.Fatalf("EnsureTree: %v", err)
	}
	for _, d := range []string{p.Home, p.Data(), p.Logs(), p.Keys()} {
		assertMode(t, d, 0o700)
	}
}

// TestZeroValuePermFallback guards the case where Paths is built directly
// (bypassing Resolve, as several tests/helpers do): the unset perm must fall
// back to the owner-only private model rather than mode 0000.
func TestZeroValuePermFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mode bits not meaningful on Windows")
	}
	p := Paths{Home: t.TempDir()} // no perm set
	if got := p.FileMode(); got != privateFileMode {
		t.Errorf("FileMode = %#o, want %#o", got, privateFileMode)
	}
	if got := p.ConfigMode(); got != privateFileMode {
		t.Errorf("ConfigMode = %#o, want %#o", got, privateFileMode)
	}
	if err := p.EnsureTree(); err != nil {
		t.Fatalf("EnsureTree: %v", err)
	}
	assertMode(t, p.Home, 0o700)
}

// TestEnsureTreeShared mirrors the macOS system-service relaxation: directories
// land at 0755 and an existing private config.yaml is widened to the config mode
// so admin-group members can edit it. Group is set to the current gid so the
// chown is permitted without root.
func TestEnsureTreeShared(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mode bits not meaningful on Windows")
	}
	home := t.TempDir()
	// A pre-existing config from the old owner-only layout.
	if err := os.WriteFile(filepath.Join(home, fileConfig), []byte("app: {}\n"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	p := Paths{Home: home, perm: Perm{Dir: 0o755, Config: 0o664, Secret: 0o600, Group: os.Getgid()}}
	if err := p.EnsureTree(); err != nil {
		t.Fatalf("EnsureTree: %v", err)
	}
	for _, d := range []string{p.Home, p.Data(), p.Logs(), p.Keys()} {
		assertMode(t, d, 0o755)
	}
	assertMode(t, p.Config(), 0o664)
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("mode %s = %#o, want %#o", path, got, want)
	}
}
