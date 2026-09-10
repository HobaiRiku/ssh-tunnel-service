package skill

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// AGENTS.md is the fallback path, for agents that read a single always-on
// instruction file instead of loading Agent Skills on demand. Because that file
// belongs to the user and is read in full on every turn, the injected block is
// delimited by markers, kept short, and points at `skill print` for the rest.
const (
	beginMarker = "<!-- BEGIN ssh-tunnel"
	endMarker   = "<!-- END ssh-tunnel -->"
)

// blockRe matches a previously injected block, whatever version it carries.
var blockRe = regexp.MustCompile(`(?s)<!-- BEGIN ssh-tunnel[^>]*-->.*?<!-- END ssh-tunnel -->`)

// AgentsPath returns the AGENTS.md to inject into: the one at the repository
// root.
//
// Deliberately never the user-home AGENTS.md that some agents also read. That
// file is the user's global instruction set; appending to it would put tunnel
// documentation into the context of every unrelated task they run.
func AgentsPath() (string, error) {
	root, err := repoRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "AGENTS.md"), nil
}

// agentsBlock renders the marker-delimited section for AGENTS.md: the skill
// body with its frontmatter removed, since the frontmatter only means something
// to clients that match against `description` before loading.
func agentsBlock(version string) (string, error) {
	body, err := Body()
	if err != nil {
		return "", err
	}
	body = strings.TrimSpace(body)
	body += "\n\nFor the full command and field reference, run `ssh-tunnel skill print --reference`."

	var b strings.Builder
	fmt.Fprintf(&b, "%s %s -->\n", beginMarker, version)
	b.WriteString(body)
	b.WriteString("\n" + endMarker + "\n")
	return b.String(), nil
}

// InjectAgents writes the block into path, creating the file when absent,
// replacing an existing block in place, and otherwise appending.
//
// Content outside the markers is preserved byte for byte: this file is the
// user's, and we only own the region between our own markers.
func InjectAgents(path string, opts Options) (Outcome, error) {
	block, err := agentsBlock(opts.Version)
	if err != nil {
		return "", err
	}

	existing, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		if opts.DryRun {
			return OutcomeInstalled, nil
		}
		if err := os.WriteFile(path, []byte(block), fileMode); err != nil {
			return "", fmt.Errorf("write %s: %w", path, err)
		}
		return OutcomeInstalled, nil
	} else if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}

	updated, outcome, err := replaceBlock(string(existing), block)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	if outcome == OutcomeUnchanged || opts.DryRun {
		return outcome, nil
	}
	if err := os.WriteFile(path, []byte(updated), fileMode); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return outcome, nil
}

// RemoveAgents strips a previously injected block, leaving the rest untouched.
// A file left holding nothing but whitespace is deleted rather than kept empty.
func RemoveAgents(path string, opts Options) (Outcome, error) {
	existing, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return OutcomeMissing, nil
	} else if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}

	found := blockRe.FindAllString(string(existing), -1)
	if len(found) == 0 {
		return OutcomeMissing, nil
	}
	if len(found) > 1 {
		return "", fmt.Errorf("%s: found %d ssh-tunnel blocks; remove them by hand", path, len(found))
	}
	if opts.DryRun {
		return OutcomeRemoved, nil
	}

	rest := strings.TrimSpace(blockRe.ReplaceAllString(string(existing), ""))
	if rest == "" {
		if err := os.Remove(path); err != nil {
			return "", fmt.Errorf("remove %s: %w", path, err)
		}
		return OutcomeRemoved, nil
	}
	if err := os.WriteFile(path, []byte(rest+"\n"), fileMode); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return OutcomeRemoved, nil
}

// replaceBlock splices block into existing, reporting what it did.
//
// Multiple blocks are an error rather than a guess: picking one would silently
// drop the other, and the user is better placed to say which is current.
func replaceBlock(existing, block string) (string, Outcome, error) {
	found := blockRe.FindAllStringIndex(existing, -1)
	switch len(found) {
	case 0:
		trimmed := strings.TrimRight(existing, "\n")
		if trimmed == "" {
			return block, OutcomeInstalled, nil
		}
		return trimmed + "\n\n" + block, OutcomeUpdated, nil
	case 1:
		lo, hi := found[0][0], found[0][1]
		if strings.TrimRight(existing[lo:hi], "\n") == strings.TrimRight(block, "\n") {
			return existing, OutcomeUnchanged, nil
		}
		return existing[:lo] + strings.TrimRight(block, "\n") + existing[hi:], OutcomeUpdated, nil
	default:
		return "", "", fmt.Errorf("found %d ssh-tunnel blocks; remove the stale ones by hand", len(found))
	}
}
