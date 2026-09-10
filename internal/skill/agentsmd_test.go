package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripFrontmatter(t *testing.T) {
	cases := map[string]struct{ in, want string }{
		"removes block":       {"---\nname: x\ndescription: y\n---\nbody\n", "body\n"},
		"leaves plain text":   {"# Title\n\nbody\n", "# Title\n\nbody\n"},
		"leaves unterminated": {"---\nname: x\nbody\n", "---\nname: x\nbody\n"},
	}
	for name, tc := range cases {
		if got := stripFrontmatter(tc.in); got != tc.want {
			t.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}
}

func TestEmbeddedBodyHasNoFrontmatter(t *testing.T) {
	body, err := Body()
	if err != nil {
		t.Fatalf("Body: %v", err)
	}
	if strings.HasPrefix(strings.TrimSpace(body), "---") {
		t.Errorf("AGENTS.md body still carries frontmatter")
	}
	if strings.Contains(body, "description:") {
		t.Errorf("AGENTS.md body leaked a frontmatter field")
	}
}

func TestInjectAgents_CreatesThenReplacesInPlace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")

	if got, err := InjectAgents(path, Options{Version: "1.0.0"}); err != nil || got != OutcomeInstalled {
		t.Fatalf("create: outcome=%q err=%v", got, err)
	}
	if got, err := InjectAgents(path, Options{Version: "1.0.0"}); err != nil || got != OutcomeUnchanged {
		t.Fatalf("reinject: outcome=%q err=%v", got, err)
	}

	if got, err := InjectAgents(path, Options{Version: "2.0.0"}); err != nil || got != OutcomeUpdated {
		t.Fatalf("upgrade: outcome=%q err=%v", got, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), endMarker); n != 1 {
		t.Fatalf("found %d blocks after upgrade, want 1", n)
	}
	if !strings.Contains(string(data), "BEGIN ssh-tunnel 2.0.0") {
		t.Errorf("version marker was not updated")
	}
}

func TestInjectAgents_PreservesSurroundingContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	original := "# AGENTS.md\n\nHouse rules the user wrote.\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := InjectAgents(path, Options{Version: "1.0.0"}); err != nil {
		t.Fatalf("inject: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(data), original) {
		t.Fatalf("existing content was not preserved verbatim:\n%s", data)
	}

	// A later version must rewrite only the marked region.
	if _, err := InjectAgents(path, Options{Version: "2.0.0"}); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	data, _ = os.ReadFile(path)
	if !strings.HasPrefix(string(data), original) {
		t.Fatalf("upgrade disturbed content outside the markers:\n%s", data)
	}
}

func TestRemoveAgents_StripsBlockKeepsRest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	original := "# AGENTS.md\n\nHouse rules.\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InjectAgents(path, Options{Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}

	if got, err := RemoveAgents(path, Options{}); err != nil || got != OutcomeRemoved {
		t.Fatalf("remove: outcome=%q err=%v", got, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file was deleted despite holding user content: %v", err)
	}
	if strings.TrimSpace(string(data)) != strings.TrimSpace(original) {
		t.Fatalf("leftover content = %q, want %q", data, original)
	}
}

func TestRemoveAgents_DeletesFileItCreated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	if _, err := InjectAgents(path, Options{Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	if got, err := RemoveAgents(path, Options{}); err != nil || got != OutcomeRemoved {
		t.Fatalf("remove: outcome=%q err=%v", got, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a file holding nothing but our block should be deleted")
	}
}

func TestReplaceBlock_RejectsDuplicates(t *testing.T) {
	doubled := beginMarker + " 1.0.0 -->\na\n" + endMarker + "\n\n" +
		beginMarker + " 1.0.0 -->\nb\n" + endMarker + "\n"
	if _, _, err := replaceBlock(doubled, "x"); err == nil {
		t.Fatal("want an error for two blocks, got nil")
	}
}
