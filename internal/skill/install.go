package skill

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// Directory and file permissions for installed payloads. A skill is
// documentation the user is expected to read and edit, not secret material —
// deliberately not paths.Secret.
const (
	dirMode  fs.FileMode = 0o755
	fileMode fs.FileMode = 0o644
)

// Outcome describes what an install did, so the caller can report it without
// re-inspecting the filesystem.
type Outcome string

const (
	// OutcomeInstalled means the destination did not exist before.
	OutcomeInstalled Outcome = "installed"
	// OutcomeUpdated means an older version of this payload was replaced.
	OutcomeUpdated Outcome = "updated"
	// OutcomeUnchanged means the destination already held this exact payload.
	OutcomeUnchanged Outcome = "up to date"
	// OutcomeRemoved is reported by Uninstall.
	OutcomeRemoved Outcome = "removed"
	// OutcomeMissing is reported by Uninstall when there was nothing to remove.
	OutcomeMissing Outcome = "not installed"
	// OutcomeWouldWrite is reported by a dry run for a destination that needs
	// --force: it holds foreign or hand-edited content, so a real run would
	// refuse rather than write.
	OutcomeWouldWrite Outcome = "would write"
	// OutcomeWouldInstall, OutcomeWouldUpdate and OutcomeWouldRemove are the
	// dry-run counterparts of the outcomes a real run would report.
	OutcomeWouldInstall Outcome = "would install"
	OutcomeWouldUpdate  Outcome = "would update"
	OutcomeWouldRemove  Outcome = "would remove"
)

// ErrForeign is returned when the destination holds content this tool did not
// write, or content a user has edited since. Overwriting either would discard
// someone's work silently, so it takes an explicit --force.
var ErrForeign = errors.New("destination was not installed by ssh-tunnel, or has local edits")

// manifest is the .ssh-tunnel-install.json dropped beside the payload. Digests
// let a later install distinguish "same content" from "user edited it".
type manifest struct {
	Version string            `json:"version"`
	Files   map[string]string `json:"files"`
}

// Options controls one install.
type Options struct {
	// Version is stamped into the manifest, normally the CLI version.
	Version string
	// Force overwrites a destination that fails the ownership check.
	Force bool
	// DryRun reports what would happen without touching the filesystem.
	DryRun bool
}

// Install writes fsys into dir, replacing whatever this tool put there before.
//
// It refuses a destination it does not recognise unless opts.Force is set: a
// directory with no manifest was created by someone else, and one whose files
// no longer match its manifest has been edited by hand.
func Install(dir string, fsys fs.FS, opts Options) (Outcome, error) {
	want, err := collect(fsys)
	if err != nil {
		return "", err
	}

	state, err := inspect(dir, want)
	if err != nil {
		return "", err
	}
	switch state {
	case stateClean:
		return OutcomeUnchanged, nil
	case stateForeign:
		if !opts.Force {
			return "", fmt.Errorf("%s: %w; pass --force to overwrite", dir, ErrForeign)
		}
	}

	if opts.DryRun {
		return OutcomeWouldWrite, nil
	}

	if err := writeAtomic(dir, want, opts.Version); err != nil {
		return "", err
	}
	if state == stateAbsent {
		return OutcomeInstalled, nil
	}
	return OutcomeUpdated, nil
}

// Uninstall removes a payload this tool installed. Like Install it refuses a
// destination it does not own without Force.
func Uninstall(dir string, fsys fs.FS, opts Options) (Outcome, error) {
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return OutcomeMissing, nil
	} else if err != nil {
		return "", fmt.Errorf("stat %s: %w", dir, err)
	}

	want, err := collect(fsys)
	if err != nil {
		return "", err
	}
	state, err := inspect(dir, want)
	if err != nil {
		return "", err
	}
	if state == stateForeign && !opts.Force {
		return "", fmt.Errorf("%s: %w; pass --force to remove anyway", dir, ErrForeign)
	}

	if opts.DryRun {
		return OutcomeWouldRemove, nil
	}
	if err := os.RemoveAll(dir); err != nil {
		return "", fmt.Errorf("remove %s: %w", dir, err)
	}
	pruneEmptyParents(dir)
	return OutcomeRemoved, nil
}

// installState classifies a destination directory.
type installState int

const (
	stateAbsent  installState = iota // nothing there
	stateClean                       // holds exactly the payload we would write
	stateOurs                        // ours, unmodified, but an older payload
	stateForeign                     // not ours, or ours with local edits
)

// inspect compares a destination against the payload we intend to write.
func inspect(dir string, want payload) (installState, error) {
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return stateAbsent, nil
	} else if err != nil {
		return 0, fmt.Errorf("stat %s: %w", dir, err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, manifestFile))
	if errors.Is(err, fs.ErrNotExist) {
		return stateForeign, nil
	} else if err != nil {
		return 0, fmt.Errorf("read %s: %w", manifestFile, err)
	}
	var m manifest
	if json.Unmarshal(raw, &m) != nil || m.Files == nil {
		return stateForeign, nil
	}

	// Every file the manifest claims must still be on disk with the recorded
	// digest; anything else means a hand edit.
	for name, recorded := range m.Files {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil || digest(data) != recorded {
			return stateForeign, nil
		}
	}

	if sameDigests(m.Files, want.digests()) {
		return stateClean, nil
	}
	return stateOurs, nil
}

func sameDigests(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// writeAtomic materialises the payload in a sibling temporary directory and
// swaps it in, so an interrupted install never leaves a half-written skill
// (a SKILL.md whose reference.md is missing is worse than no skill at all).
func writeAtomic(dir string, want payload, version string) error {
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, dirMode); err != nil {
		return fmt.Errorf("create %s: %w", parent, err)
	}
	tmp, err := os.MkdirTemp(parent, "."+filepath.Base(dir)+".tmp-")
	if err != nil {
		return fmt.Errorf("create staging directory in %s: %w", parent, err)
	}
	defer os.RemoveAll(tmp)

	for _, name := range want.paths() {
		dst := filepath.Join(tmp, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(dst), dirMode); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(dst), err)
		}
		if err := os.WriteFile(dst, want[name], fileMode); err != nil {
			return fmt.Errorf("write %s: %w", dst, err)
		}
	}

	m := manifest{Version: version, Files: want.digests()}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, manifestFile), append(raw, '\n'), fileMode); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	// os.Rename cannot replace a non-empty directory, so the old payload goes
	// first. The gap is small and both sides are reconstructible by re-running
	// install, which is why the manifest is written last inside the staging
	// directory: a crash mid-swap leaves an unmanifested directory, which the
	// next run classifies as foreign rather than silently overwriting.
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("replace %s: %w", dir, err)
	}
	if err := os.Rename(tmp, dir); err != nil {
		return fmt.Errorf("move staged payload into %s: %w", dir, err)
	}
	if err := os.Chmod(dir, dirMode); err != nil {
		return fmt.Errorf("chmod %s: %w", dir, err)
	}
	return nil
}

// pruneEmptyParents removes the `skills` directory, and the client root above
// it, when uninstalling left them empty — but never removes a directory that
// still holds another client's or another tool's content.
func pruneEmptyParents(dir string) {
	for i, p := 0, filepath.Dir(dir); i < 2; i, p = i+1, filepath.Dir(p) {
		entries, err := os.ReadDir(p)
		if err != nil || len(entries) > 0 {
			return
		}
		if os.Remove(p) != nil {
			return
		}
	}
}

// Plan reports, without writing, what Install would do at each destination.
// Used by `--dry-run` so the user sees every target before any of them change.
func Plan(dirs []string, fsys fs.FS) (map[string]Outcome, error) {
	want, err := collect(fsys)
	if err != nil {
		return nil, err
	}
	out := make(map[string]Outcome, len(dirs))
	for _, dir := range dirs {
		state, err := inspect(dir, want)
		if err != nil {
			return nil, err
		}
		switch state {
		case stateAbsent:
			out[dir] = OutcomeWouldInstall
		case stateClean:
			out[dir] = OutcomeUnchanged
		case stateOurs:
			out[dir] = OutcomeWouldUpdate
		default:
			out[dir] = OutcomeWouldWrite
		}
	}
	return out, nil
}

// Files lists the payload paths that would be written, for dry-run output.
func Files(fsys fs.FS) ([]string, error) {
	want, err := collect(fsys)
	if err != nil {
		return nil, err
	}
	names := append(want.paths(), manifestFile)
	sort.Strings(names)
	return names, nil
}
