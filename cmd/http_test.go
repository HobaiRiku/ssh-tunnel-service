package cmd

import "testing"

func TestNormalizeListen(t *testing.T) {
	cases := map[string]string{
		"127.0.0.1:2222": "127.0.0.1:2222",
		"0.0.0.0:2222":   "127.0.0.1:2222",
		":2222":          "127.0.0.1:2222",
		"[::]:2222":      "127.0.0.1:2222",
		"[::1]:2222":     "[::1]:2222",
		"localhost:2222": "localhost:2222",
		"noport":         "noport",
	}
	for in, want := range cases {
		if got := normalizeListen(in); got != want {
			t.Errorf("normalizeListen(%q) = %q, want %q", in, got, want)
		}
	}
}
