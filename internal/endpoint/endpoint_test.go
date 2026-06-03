package endpoint

import (
	"testing"
)

func TestNormalizeAddr(t *testing.T) {
	cases := map[string]string{
		"127.0.0.1:2222": "127.0.0.1:2222",
		"0.0.0.0:2222":   "127.0.0.1:2222",
		":2222":          "127.0.0.1:2222",
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
// advertises under the user scope (redirected here via XDG_RUNTIME_DIR), while a
// root run advertises under the system scope.
func TestWriteDiscoverRoundtrip(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

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
