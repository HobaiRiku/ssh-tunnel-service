package skill

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func testPayload() fstest.MapFS {
	return fstest.MapFS{
		"SKILL.md":     {Data: []byte("---\nname: x\n---\nbody\n")},
		"reference.md": {Data: []byte("ref\n")},
	}
}

func TestInstall_FreshThenIdempotent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skills", Name)
	fsys := testPayload()

	got, err := Install(dir, fsys, Options{Version: "1.0.0"})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if got != OutcomeInstalled {
		t.Fatalf("outcome = %q, want %q", got, OutcomeInstalled)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "SKILL.md")); err != nil || !strings.Contains(string(data), "body") {
		t.Fatalf("SKILL.md not written: %v", err)
	}

	got, err = Install(dir, fsys, Options{Version: "1.0.0"})
	if err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	if got != OutcomeUnchanged {
		t.Fatalf("second outcome = %q, want %q", got, OutcomeUnchanged)
	}
}

func TestInstall_UpgradeReplacesRemovedFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skills", Name)
	if _, err := Install(dir, testPayload(), Options{Version: "1.0.0"}); err != nil {
		t.Fatalf("install: %v", err)
	}

	// A later version drops reference.md; the stale copy must not survive, or
	// the agent would keep reading content the new skill no longer ships.
	next := fstest.MapFS{"SKILL.md": {Data: []byte("---\nname: x\n---\nnewer\n")}}
	got, err := Install(dir, next, Options{Version: "1.1.0"})
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if got != OutcomeUpdated {
		t.Fatalf("outcome = %q, want %q", got, OutcomeUpdated)
	}
	if _, err := os.Stat(filepath.Join(dir, "reference.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale reference.md survived the upgrade")
	}
}

func TestInstall_RefusesForeignDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skills", Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	handwritten := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(handwritten, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(dir, testPayload(), Options{Version: "1.0.0"}); !errors.Is(err, ErrForeign) {
		t.Fatalf("err = %v, want ErrForeign", err)
	}
	if data, _ := os.ReadFile(handwritten); string(data) != "mine\n" {
		t.Fatalf("refused install still modified the file")
	}

	if _, err := Install(dir, testPayload(), Options{Version: "1.0.0", Force: true}); err != nil {
		t.Fatalf("forced install: %v", err)
	}
}

func TestInstall_RefusesLocallyEditedInstall(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skills", Name)
	if _, err := Install(dir, testPayload(), Options{Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("edited by hand\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	next := fstest.MapFS{"SKILL.md": {Data: []byte("v2\n")}}
	if _, err := Install(dir, next, Options{Version: "1.1.0"}); !errors.Is(err, ErrForeign) {
		t.Fatalf("err = %v, want ErrForeign for a hand-edited install", err)
	}
}

func TestInstall_DryRunWritesNothing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skills", Name)
	got, err := Install(dir, testPayload(), Options{Version: "1.0.0", DryRun: true})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if got != OutcomeWouldWrite {
		t.Fatalf("outcome = %q, want %q", got, OutcomeWouldWrite)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry run created %s", dir)
	}
}

func TestUninstall_RemovesAndPrunes(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".claude", "skills", Name)
	if _, err := Install(dir, testPayload(), Options{Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}

	got, err := Uninstall(dir, testPayload(), Options{})
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if got != OutcomeRemoved {
		t.Fatalf("outcome = %q, want %q", got, OutcomeRemoved)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("empty skills directory was not pruned")
	}

	got, err = Uninstall(dir, testPayload(), Options{})
	if err != nil {
		t.Fatalf("second uninstall: %v", err)
	}
	if got != OutcomeMissing {
		t.Fatalf("outcome = %q, want %q", got, OutcomeMissing)
	}
}

func TestUninstall_KeepsSiblingContent(t *testing.T) {
	root := t.TempDir()
	skills := filepath.Join(root, ".claude", "skills")
	dir := filepath.Join(skills, Name)
	if _, err := Install(dir, testPayload(), Options{Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(skills, "someone-elses-skill")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Uninstall(dir, testPayload(), Options{}); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("pruning removed an unrelated skill: %v", err)
	}
}

func TestEmbeddedSkillIsInstallable(t *testing.T) {
	fsys, err := SkillFS()
	if err != nil {
		t.Fatalf("SkillFS: %v", err)
	}
	dir := filepath.Join(t.TempDir(), Name)
	if _, err := Install(dir, fsys, Options{Version: "test"}); err != nil {
		t.Fatalf("install embedded payload: %v", err)
	}
	for _, name := range []string{"SKILL.md", "reference.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("embedded payload is missing %s: %v", name, err)
		}
	}
}
