package skill

import (
	"fmt"
	"os"
	"path/filepath"
)

// Client identifies an agent client whose skill discovery root we know.
//
// The Agent Skills format is shared, but each client keeps its own root and
// none of them scans another's — so a "target" is exactly a (client, scope)
// pair resolved to a directory.
type Client string

const (
	// ClaudeCode discovers skills in <root>/skills/<name>/, where <root> is
	// ~/.claude (or $CLAUDE_CONFIG_DIR) for user scope and <repo>/.claude for
	// project scope.
	ClaudeCode Client = "claude"
	// Codex discovers skills in ~/.agents/skills/<name>/ (user scope) and
	// <repo>/.agents/skills/<name>/ (project scope).
	Codex Client = "codex"
)

// Clients lists every client `--target` accepts, in install order.
var Clients = []Client{ClaudeCode, Codex}

// Scope is user-level (every project) versus repository-level.
type Scope string

const (
	ScopeUser    Scope = "user"
	ScopeProject Scope = "project"
)

// Target is one resolved destination for the skill payload.
type Target struct {
	Client Client
	Scope  Scope
	// Root is the client's configuration root (~/.claude, <repo>/.agents, …).
	// Detection keys off this rather than Dir: the skills subdirectory usually
	// does not exist until the first install, so testing for it would mean the
	// client is never detected.
	Root string
	// Dir is the skill directory itself, the one that ends up holding SKILL.md.
	Dir string
}

// String renders a target for CLI output, e.g. `claude (user)`.
func (t Target) String() string { return fmt.Sprintf("%s (%s)", t.Client, t.Scope) }

// Detected reports whether the client's root already exists, which is the
// signal that this client is actually installed on the machine.
func (t Target) Detected() bool {
	info, err := os.Stat(t.Root)
	return err == nil && info.IsDir()
}

// clientRoot returns the client configuration directory name used under a
// repository, and for user scope the absolute root.
func clientRoot(c Client, scope Scope, base string) (string, error) {
	switch c {
	case ClaudeCode:
		if scope == ScopeUser {
			// A relocated config dir is common (dotfiles, multi-account
			// setups). Missing this variable installs into a directory the
			// client never reads, and nothing reports an error.
			if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
				return dir, nil
			}
		}
		return filepath.Join(base, ".claude"), nil
	case Codex:
		return filepath.Join(base, ".agents"), nil
	default:
		return "", fmt.Errorf("unknown target %q (want one of: claude, codex)", c)
	}
}

// ResolveTarget maps a client and scope to a concrete destination.
//
// User scope resolves against the invoking user's home directory. Project
// scope resolves against the repository root discovered from cwd, not cwd
// itself: a skill written to a subdirectory's .claude/ or .agents/ is silently
// ignored by both clients.
func ResolveTarget(c Client, scope Scope) (Target, error) {
	var base string
	var err error
	switch scope {
	case ScopeUser:
		base, err = os.UserHomeDir()
		if err != nil || base == "" {
			return Target{}, fmt.Errorf("resolve user home: %w", err)
		}
	case ScopeProject:
		base, err = repoRoot()
		if err != nil {
			return Target{}, err
		}
	default:
		return Target{}, fmt.Errorf("unknown scope %q", scope)
	}

	root, err := clientRoot(c, scope, base)
	if err != nil {
		return Target{}, err
	}
	return Target{
		Client: c,
		Scope:  scope,
		Root:   root,
		Dir:    filepath.Join(root, "skills", Name),
	}, nil
}

// ResolveTargets resolves every requested client at one scope. With no clients
// named it auto-detects: every client whose root already exists is selected,
// and it is an error if none is.
func ResolveTargets(clients []Client, scope Scope) ([]Target, error) {
	if len(clients) > 0 {
		out := make([]Target, 0, len(clients))
		for _, c := range clients {
			t, err := ResolveTarget(c, scope)
			if err != nil {
				return nil, err
			}
			out = append(out, t)
		}
		return out, nil
	}

	var detected []Target
	for _, c := range Clients {
		t, err := ResolveTarget(c, scope)
		if err != nil {
			// A client whose root cannot even be resolved (no repository for
			// project scope) is reported; a merely absent root is not.
			return nil, err
		}
		if t.Detected() {
			detected = append(detected, t)
		}
	}
	if len(detected) == 0 {
		return nil, fmt.Errorf(
			"no agent client detected at %s scope; pass --target claude|codex to install anyway, or --dir <path> for a client not listed",
			scope)
	}
	return detected, nil
}

// repoRoot walks up from the working directory to the nearest ancestor holding
// a .git entry.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not inside a git repository; use --dir <path> to choose the destination explicitly")
		}
		dir = parent
	}
}

// ParseClient maps a --target value to a Client.
func ParseClient(s string) (Client, error) {
	switch Client(s) {
	case ClaudeCode:
		return ClaudeCode, nil
	case Codex:
		return Codex, nil
	default:
		return "", fmt.Errorf("unknown target %q (want one of: claude, codex, all)", s)
	}
}
