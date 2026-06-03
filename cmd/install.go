package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"ssh-tunnel-service/internal/elevate"
	"ssh-tunnel-service/internal/service"
)

func installCmd() *cobra.Command {
	var importKeys []string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the system service",
		Long: "Install ssh-tunnel-service as a system service.\n\n" +
			"Requires administrator privileges; you will be prompted to elevate if needed.\n" +
			"When run interactively, you may import private keys from your ~/.ssh into the\n" +
			"system key store so the root-owned service can authenticate. A managed default\n" +
			"key is always generated regardless of import.",
		RunE: func(_ *cobra.Command, _ []string) error {
			// Before elevating, when interactive and the caller didn't already
			// specify keys, offer to import the current user's ~/.ssh keys. The
			// choices are forwarded to the elevated child as --import-key flags.
			if !elevate.IsElevated() && len(importKeys) == 0 {
				importKeys = promptKeyImport()
			}
			extra := make([]string, 0, len(importKeys))
			for _, k := range importKeys {
				extra = append(extra, "--import-key="+k)
			}
			if handled, err := ensurePrivileged(extra...); handled {
				return err
			}

			// Elevated: register the service, then provision the system home with
			// the chosen keys and a guaranteed default key.
			if err := service.Install(rootFlags.Home); err != nil {
				return err
			}
			if err := service.Provision(rootFlags.Home, importKeys); err != nil {
				return fmt.Errorf("provision keys: %w", err)
			}
			fmt.Println("Service installed successfully.")
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&importKeys, "import-key", nil,
		"private key file to import into the system key store (repeatable; skips the interactive prompt)")
	return cmd
}

// promptKeyImport scans the invoking user's ~/.ssh for private keys and asks
// which to import. Returns nil when not interactive or nothing is selected.
func promptKeyImport() []string {
	if !stdinIsTTY() {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	candidates := discoverSSHKeys(filepath.Join(home, ".ssh"))
	if len(candidates) == 0 {
		return nil
	}

	fmt.Fprintln(os.Stderr, "Found SSH private keys in ~/.ssh:")
	for i, c := range candidates {
		fmt.Fprintf(os.Stderr, "  [%d] %s\n", i+1, c)
	}
	fmt.Fprint(os.Stderr, "Import which keys into the system service? "+
		"(comma-separated numbers, empty = none, a default key is still generated): ")

	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return parseSelection(line, candidates)
}

// discoverSSHKeys lists private-key files in dir (excluding .pub, config, and
// other non-key files), sorted for stable presentation.
func discoverSSHKeys(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".pub") {
			continue
		}
		full := filepath.Join(dir, e.Name())
		if looksLikePrivateKey(full) {
			out = append(out, full)
		}
	}
	sort.Strings(out)
	return out
}

func looksLikePrivateKey(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 64)
	n, _ := f.Read(buf)
	return strings.Contains(string(buf[:n]), "PRIVATE KEY")
}

// parseSelection maps a comma-separated list of 1-based indices to paths,
// ignoring blanks and out-of-range entries, de-duplicating the result.
func parseSelection(line string, candidates []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, tok := range strings.Split(strings.TrimSpace(line), ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		n, err := strconv.Atoi(tok)
		if err != nil || n < 1 || n > len(candidates) {
			continue
		}
		path := candidates[n-1]
		if !seen[path] {
			seen[path] = true
			out = append(out, path)
		}
	}
	return out
}

func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
