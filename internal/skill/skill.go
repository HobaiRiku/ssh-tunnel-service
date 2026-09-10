// Package skill embeds the Agent Plugin payload (an Agent Skills 1.0 skill
// directory plus its plugin manifest) and installs it into the fixed
// directories that agent clients discover skills from.
//
// The payload is a single source of truth: the same SKILL.md is what Claude
// Code reads from ~/.claude/skills, what Codex reads from ~/.agents/skills,
// and what `skill print` writes to stdout. Only the destination root differs
// per client — the file format is the shared Agent Skills standard.
package skill

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Name is the skill directory name. It doubles as the identifier users type to
// invoke the skill explicitly (Codex `$ssh-tunnel`), so it matches the CLI
// binary name rather than the longer service name.
const Name = "ssh-tunnel"

// manifestFile records what this tool installed into a skill directory. It is
// a dotfile so agent clients ignore it; only `skill install` reads it, to tell
// an up-to-date install from an outdated one from a user-modified one.
const manifestFile = ".ssh-tunnel-install.json"

//go:embed all:plugin
var embedded embed.FS

// PluginFS returns the whole Agent Plugin directory: plugin.json alongside
// skills/<Name>/. Used by --plugin installs, where a client consumes the
// packaged form rather than a bare skill directory.
func PluginFS() (fs.FS, error) {
	sub, err := fs.Sub(embedded, "plugin")
	if err != nil {
		return nil, fmt.Errorf("open embedded plugin: %w", err)
	}
	return sub, nil
}

// SkillFS returns just the skill directory (SKILL.md + reference.md). This is
// what user-level installs copy: neither Claude Code nor Codex reads
// plugin.json out of their skills roots.
func SkillFS() (fs.FS, error) {
	sub, err := fs.Sub(embedded, path.Join("plugin", "skills", Name))
	if err != nil {
		return nil, fmt.Errorf("open embedded skill: %w", err)
	}
	return sub, nil
}

// File reads one file out of the embedded skill directory.
func File(name string) ([]byte, error) {
	fsys, err := SkillFS()
	if err != nil {
		return nil, err
	}
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, fmt.Errorf("read embedded %s: %w", name, err)
	}
	return data, nil
}

// payload is a flattened snapshot of an fs.FS: relative path → contents.
type payload map[string][]byte

// collect flattens fsys into a payload, skipping nothing — the embedded tree
// only holds files we intend to ship.
func collect(fsys fs.FS) (payload, error) {
	out := payload{}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		out[p] = data
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read embedded payload: %w", err)
	}
	return out, nil
}

// paths returns the payload's file paths in stable order.
func (p payload) paths() []string {
	out := make([]string, 0, len(p))
	for k := range p {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// digests maps each payload path to the hex sha256 of its contents.
func (p payload) digests() map[string]string {
	out := make(map[string]string, len(p))
	for k, v := range p {
		out[k] = digest(v)
	}
	return out
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Body returns SKILL.md with its YAML frontmatter stripped. The frontmatter is
// meaningful only to clients that load Agent Skills on demand (it carries the
// `description` they match against); a file that is always read in full, such
// as AGENTS.md, must not carry it.
func Body() (string, error) {
	data, err := File("SKILL.md")
	if err != nil {
		return "", err
	}
	return stripFrontmatter(string(data)), nil
}

// stripFrontmatter removes a leading `---`-delimited YAML block. Text without
// one is returned unchanged, so this is safe to apply unconditionally.
func stripFrontmatter(s string) string {
	s = strings.TrimPrefix(s, "\ufeff")
	if !strings.HasPrefix(s, "---\n") {
		return s
	}
	rest := s[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return s
	}
	return strings.TrimLeft(rest[end+len("\n---\n"):], "\n")
}
