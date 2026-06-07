package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureInstallHomeCreatesDataTree(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")

	if err := ensureInstallHome(home); err != nil {
		t.Fatalf("ensureInstallHome: %v", err)
	}

	for _, dir := range []string{
		home,
		filepath.Join(home, "data"),
		filepath.Join(home, "logs"),
		filepath.Join(home, "keys"),
	} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %s: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a directory", dir)
		}
	}
}
