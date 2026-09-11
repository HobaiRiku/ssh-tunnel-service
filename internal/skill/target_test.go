package skill

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// homeDir points os.UserHomeDir at a scratch directory for the test.
func homeDir(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("home override differs on Windows")
	}
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	return dir
}

func TestResolveTarget_UserScopeRoots(t *testing.T) {
	home := homeDir(t)

	claude, err := ResolveTarget(ClaudeCode, ScopeUser)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".claude", "skills", Name); claude.Dir != want {
		t.Errorf("claude dir = %q, want %q", claude.Dir, want)
	}

	codex, err := ResolveTarget(Codex, ScopeUser)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".agents", "skills", Name); codex.Dir != want {
		t.Errorf("codex dir = %q, want %q", codex.Dir, want)
	}
}

// A relocated Claude config dir must be honoured: installing into ~/.claude
// anyway would land the skill somewhere the client never reads, with no error.
func TestResolveTarget_HonoursClaudeConfigDir(t *testing.T) {
	homeDir(t)
	custom := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", custom)

	got, err := ResolveTarget(ClaudeCode, ScopeUser)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(custom, "skills", Name); got.Dir != want {
		t.Errorf("dir = %q, want %q", got.Dir, want)
	}
}

// CLAUDE_CONFIG_DIR names a user-level root and must not hijack a project
// install, which belongs to the repository being worked in.
func TestResolveTarget_ProjectScopeIgnoresClaudeConfigDir(t *testing.T) {
	homeDir(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	repo := gitRepo(t)

	got, err := ResolveTarget(ClaudeCode, ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(repo, ".claude", "skills", Name); got.Dir != want {
		t.Errorf("dir = %q, want %q", got.Dir, want)
	}
}

// gitRepo creates a repository with a nested working directory and chdirs into
// the nested one, so tests exercise the walk up to the root.
func gitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(repo, "ui", "src")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)
	// Derive the expected root from the working directory the product will
	// actually see: a macOS temp dir is reachable as both /var/… and
	// /private/var/…, and repoRoot walks up from whichever spelling os.Getwd
	// returns. Comparing against the other spelling fails on a real path.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(filepath.Dir(wd))
}

// Resolving from a subdirectory must find the repository root: both clients
// ignore a .claude/ or .agents/ that is not at the top of the repository.
func TestResolveTarget_ProjectScopeWalksToRepoRoot(t *testing.T) {
	homeDir(t)
	repo := gitRepo(t)

	got, err := ResolveTarget(Codex, ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(repo, ".agents", "skills", Name); got.Dir != want {
		t.Errorf("dir = %q, want %q", got.Dir, want)
	}
}

func TestResolveTargets_AutoDetectsExistingRootsOnly(t *testing.T) {
	home := homeDir(t)
	if err := os.MkdirAll(filepath.Join(home, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveTargets(nil, ScopeUser)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Client != Codex {
		t.Fatalf("got %v, want only codex", got)
	}
}

// Detection keys off the client root, not its skills subdirectory: the latter
// usually does not exist until the first install.
func TestResolveTargets_DetectsRootWithoutSkillsSubdir(t *testing.T) {
	home := homeDir(t)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveTargets(nil, ScopeUser)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Client != ClaudeCode {
		t.Fatalf("got %v, want only claude", got)
	}
}

func TestResolveTargets_NoClientDetected(t *testing.T) {
	homeDir(t)
	_, err := ResolveTargets(nil, ScopeUser)
	if err == nil {
		t.Fatal("want an error when nothing is detected")
	}
	if !strings.Contains(err.Error(), "--target") {
		t.Errorf("error should name the way out, got: %v", err)
	}
}

// An explicit --target installs even when the client is not present, so a user
// can prepare a machine before installing the agent itself.
func TestResolveTargets_ExplicitSkipsDetection(t *testing.T) {
	homeDir(t)
	got, err := ResolveTargets([]Client{ClaudeCode, Codex}, ScopeUser)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d targets, want 2", len(got))
	}
}

func TestParseClient_Unknown(t *testing.T) {
	if _, err := ParseClient("cursor"); err == nil {
		t.Fatal("want an error for an unknown client")
	}
}

func TestRepoRoot_OutsideRepository(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := repoRoot(); err == nil {
		t.Fatal("want an error outside a git repository")
	}
}
