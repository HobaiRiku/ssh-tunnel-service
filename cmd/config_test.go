package cmd

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestResolveConfigPathsUsesResolvedClientHome(t *testing.T) {
	t.Cleanup(func() { resolveClientForConfig = resolveClient })
	resolveClientForConfig = func(home string) (*apiClient, error) {
		if home != "" {
			t.Fatalf("home override = %q, want empty", home)
		}
		return &apiClient{home: "/tmp/system-home"}, nil
	}

	p, err := resolveConfigPaths("")
	if err != nil {
		t.Fatalf("resolveConfigPaths: %v", err)
	}
	if p.Home != "/tmp/system-home" {
		t.Fatalf("home = %q, want %q", p.Home, "/tmp/system-home")
	}
}

func TestResolveConfigPathsFallsBackToHomeOverride(t *testing.T) {
	t.Cleanup(func() { resolveClientForConfig = resolveClient })
	resolveClientForConfig = func(_ string) (*apiClient, error) {
		return &apiClient{}, nil
	}

	tmp := t.TempDir()
	p, err := resolveConfigPaths(tmp)
	if err != nil {
		t.Fatalf("resolveConfigPaths: %v", err)
	}
	want, err := filepath.Abs(tmp)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if p.Home != want {
		t.Fatalf("home = %q, want %q", p.Home, want)
	}
}

func TestResolveConfigPathsPropagatesResolveErrors(t *testing.T) {
	t.Cleanup(func() { resolveClientForConfig = resolveClient })
	resolveClientForConfig = func(_ string) (*apiClient, error) {
		return nil, errors.New("boom")
	}

	if _, err := resolveConfigPaths(""); err == nil {
		t.Fatal("resolveConfigPaths error = nil, want non-nil")
	}
}
