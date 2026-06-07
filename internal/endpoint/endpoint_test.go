package endpoint

import (
	"encoding/json"
	"os"
	"testing"
)

func TestNormalizeAddr(t *testing.T) {
	cases := map[string]string{
		"127.0.0.1:2222": "127.0.0.1:2222",
		"0.0.0.0:2222":   "127.0.0.1:2222",
		":2222":          "127.0.0.1:2222",
		"[::]:2222":      "127.0.0.1:2222",
		"[::1]:2222":     "[::1]:2222",
		"192.168.1.5:80": "192.168.1.5:80",
		"noport":         "noport",
	}
	for in, want := range cases {
		if got := normalizeAddr(in); got != want {
			t.Errorf("normalizeAddr(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestWriteDiscoverRoundtrip writes an endpoint and confirms Discover reads it
// back with a normalized address. It works under either scope: a non-root run
// advertises under the user scope (redirected here into a temp dir), while a
// root run advertises under the system scope.
func TestWriteDiscoverRoundtrip(t *testing.T) {
	tmp := t.TempDir()
	// Redirect every platform's user-scope directory to tmp so the test is
	// hermetic: XDG_RUNTIME_DIR (Linux), HOME/USERPROFILE (macOS UserCacheDir
	// via ~/Library/Caches), and TEMP (Windows).
	t.Setenv("XDG_RUNTIME_DIR", tmp)
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("TEMP", tmp)
	t.Setenv("TMP", tmp)

	path, err := Write("0.0.0.0:2222", "/tmp/home")
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if path == "" {
		t.Fatal("Write returned empty path")
	}
	t.Cleanup(Remove)

	got := Discover()
	if len(got) == 0 {
		t.Fatal("Discover returned no instances")
	}
	first := got[0]
	if first.Scope != CurrentScope() {
		t.Errorf("scope = %q, want %q", first.Scope, CurrentScope())
	}
	if first.Address != "127.0.0.1:2222" {
		t.Errorf("address = %q, want normalized loopback", first.Address)
	}
	if first.Home != "/tmp/home" {
		t.Errorf("home = %q, want /tmp/home", first.Home)
	}

	Remove()
	if len(Discover()) != 0 {
		t.Error("Discover still returns an instance after Remove")
	}
}

func TestRemoveDoesNotDeleteEndpointOwnedByAnotherProcess(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmp)
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("TEMP", tmp)
	t.Setenv("TMP", tmp)

	path, err := Write("127.0.0.1:2222", "/tmp/old")
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	foreign := Info{
		Address: "127.0.0.1:3333",
		Home:    "/tmp/new",
		PID:     os.Getpid() + 1,
		Scope:   CurrentScope(),
	}
	data, err := json.Marshal(foreign)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("overwrite endpoint: %v", err)
	}

	Remove()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected foreign endpoint to remain, stat: %v", err)
	}
	got := Discover()
	if len(got) == 0 || got[0].Address != foreign.Address {
		t.Fatalf("expected discovery to keep foreign endpoint, got %+v", got)
	}
}
