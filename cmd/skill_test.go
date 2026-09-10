package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"ssh-tunnel-service/internal/skill"
)

// runSkillCmd executes the skill subcommand tree with args, returning stdout.
func runSkillCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	c := skillCmd()
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&out)
	c.SetArgs(args)
	err := c.Execute()
	return out.String(), err
}

func skillTestHome(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("home override differs on Windows")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	return home
}

func TestSkillInstall_AutoDetectInstallsIntoDetectedClientsOnly(t *testing.T) {
	home := skillTestHome(t)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}

	out, err := runSkillCmd(t, "install")
	if err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", skill.Name, "SKILL.md")); err != nil {
		t.Errorf("claude skill not installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents")); !os.IsNotExist(err) {
		t.Errorf("installed into an undetected client root")
	}
	if !strings.Contains(out, "claude (user)") {
		t.Errorf("output should name the destination, got:\n%s", out)
	}
}

func TestSkillInstall_ExplicitTargetAll(t *testing.T) {
	home := skillTestHome(t)

	if out, err := runSkillCmd(t, "install", "--target", "all"); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	for _, root := range []string{".claude", ".agents"} {
		p := filepath.Join(home, root, "skills", skill.Name, "SKILL.md")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
}

func TestSkillInstall_DryRunWritesNothing(t *testing.T) {
	home := skillTestHome(t)

	out, err := runSkillCmd(t, "install", "--target", "claude", "--dry-run")
	if err != nil {
		t.Fatalf("dry run: %v\n%s", err, out)
	}
	if !strings.Contains(out, "dry run") {
		t.Errorf("output should say it is a dry run, got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude")); !os.IsNotExist(err) {
		t.Errorf("dry run created the destination")
	}
}

func TestSkillInstall_DirIsUsedVerbatim(t *testing.T) {
	skillTestHome(t)
	dir := filepath.Join(t.TempDir(), "anywhere")

	if out, err := runSkillCmd(t, "install", "--dir", dir); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Errorf("skill not written to --dir: %v", err)
	}
}

func TestSkillInstall_PluginRequiresDir(t *testing.T) {
	skillTestHome(t)
	_, err := runSkillCmd(t, "install", "--plugin", "--target", "claude")
	if err == nil || !strings.Contains(err.Error(), "--dir") {
		t.Fatalf("err = %v, want a complaint about --dir", err)
	}
}

func TestSkillInstall_PluginWritesManifestAndSkill(t *testing.T) {
	skillTestHome(t)
	dir := filepath.Join(t.TempDir(), "plugin")

	if out, err := runSkillCmd(t, "install", "--dir", dir, "--plugin"); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	for _, rel := range []string{"plugin.json", filepath.Join("skills", skill.Name, "SKILL.md")} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
}

func TestSkillInstall_DirRejectsTargetFlags(t *testing.T) {
	skillTestHome(t)
	_, err := runSkillCmd(t, "install", "--dir", t.TempDir(), "--target", "claude")
	if err == nil || !strings.Contains(err.Error(), "--dir") {
		t.Fatalf("err = %v, want a conflict error", err)
	}
}

func TestSkillUninstall_RemovesWhatInstallWrote(t *testing.T) {
	home := skillTestHome(t)
	if out, err := runSkillCmd(t, "install", "--target", "codex"); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	if out, err := runSkillCmd(t, "uninstall", "--target", "codex"); err != nil {
		t.Fatalf("uninstall: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", skill.Name)); !os.IsNotExist(err) {
		t.Errorf("skill directory survived uninstall")
	}
}

func TestSkillInstall_AgentsMDRoundTrip(t *testing.T) {
	skillTestHome(t)
	path := filepath.Join(t.TempDir(), "AGENTS.md")

	if out, err := runSkillCmd(t, "install", "--target", "claude", "--agents-md-path", path); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("AGENTS.md not written: %v", err)
	}
	if strings.Contains(string(data), "description:") {
		t.Errorf("frontmatter leaked into AGENTS.md")
	}

	if out, err := runSkillCmd(t, "uninstall", "--target", "claude", "--agents-md-path", path); err != nil {
		t.Fatalf("uninstall: %v\n%s", err, out)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("AGENTS.md should be gone when it held nothing else")
	}
}

func TestSkillPrint(t *testing.T) {
	out, err := runSkillCmd(t, "print")
	if err != nil {
		t.Fatalf("print: %v", err)
	}
	if !strings.HasPrefix(out, "---\n") {
		t.Errorf("print should emit SKILL.md verbatim, frontmatter included")
	}

	ref, err := runSkillCmd(t, "print", "--reference")
	if err != nil {
		t.Fatalf("print --reference: %v", err)
	}
	if !strings.Contains(ref, "ssh-tunnel tunnel add") {
		t.Errorf("reference output looks wrong:\n%s", ref)
	}
}
