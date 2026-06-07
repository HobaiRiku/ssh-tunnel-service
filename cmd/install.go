package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"ssh-tunnel-service/internal/elevate"
	"ssh-tunnel-service/internal/paths"
	"ssh-tunnel-service/internal/service"
)

func installCmd() *cobra.Command {
	var (
		importKeys []string
		userScope  bool
	)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install as a system service (or --user for a per-user service)",
		Long: "Install ssh-tunnel-service.\n\n" +
			"By default it is installed as a system service (systemd system unit, macOS\n" +
			"LaunchDaemon, Windows SCM), starting at boot without an interactive login.\n" +
			"This requires administrator privileges; you will be prompted to elevate if\n" +
			"needed. You may import private keys from your ~/.ssh into the system key store\n" +
			"so the root-owned service can authenticate, and a managed default key is always\n" +
			"generated.\n\n" +
			"With --user it is installed as a per-user service (systemd --user unit, macOS\n" +
			"LaunchAgent) that runs as you without root and uses your normal ssh identities\n" +
			"(agent / ~/.ssh) — exactly like `ssh-tunnel run`. Not supported on Windows.",
		RunE: func(_ *cobra.Command, _ []string) error {
			if userScope {
				return installUser()
			}
			return installSystem(importKeys)
		},
	}
	cmd.Flags().StringArrayVar(&importKeys, "import-key", nil,
		"private key file to import into the system key store (repeatable; skips the interactive prompt)")
	cmd.Flags().BoolVar(&userScope, "user", false, "install as a per-user service (no root; uses your ssh identities)")
	return cmd
}

// installSystem installs the elevated, system-wide service with managed keys.
func installSystem(importKeys []string) error {
	// Before elevating, when interactive and the caller didn't already specify
	// keys, offer to import the current user's ~/.ssh keys. The choices are
	// forwarded to the elevated child as --import-key flags.
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

	// Elevated: register the service, then provision the system home with the
	// chosen keys and a guaranteed default key. If provisioning fails after the
	// service has been installed, roll back so a retry is clean.
	if err := service.Install(rootFlags.Home, false); err != nil {
		return err
	}
	if err := service.Provision(rootFlags.Home, importKeys); err != nil {
		_ = service.Uninstall(rootFlags.Home, false)
		return fmt.Errorf("provision keys: %w", err)
	}
	printInstallSummary(false)
	return nil
}

// installUser installs the per-user service. No elevation, no managed keys — the
// service uses the invoking user's ssh identities, like `ssh-tunnel run`.
func installUser() error {
	if !service.UserScopeSupported() {
		return fmt.Errorf("user-level services are not supported on this platform; install the system service (omit --user) or use `ssh-tunnel run`")
	}
	// A per-user service is installed for the invoking user (its home, binary,
	// and unit are derived from that identity). Running under sudo/root would
	// bake in root's paths, so reject it with guidance.
	if elevate.IsElevated() {
		return fmt.Errorf("run `install --user` without sudo/root so the service installs for your user account")
	}
	if err := service.Install(rootFlags.Home, true); err != nil {
		return err
	}
	printInstallSummary(true)
	return nil
}

// printInstallSummary tells the user where the service was installed and how to
// manage it.
func printInstallSummary(user bool) {
	scope := "system"
	if user {
		scope = "user"
	}
	fmt.Printf("Service installed successfully (%s scope).\n", scope)
	if p, err := paths.Resolve(rootFlags.Home); err == nil {
		fmt.Printf("  Home:   %s\n", p.Home)
	}
	if bin, err := service.BinaryPath(user); err == nil {
		fmt.Printf("  Binary: %s\n", bin)
	}
	if user {
		fmt.Println("  Manage: ssh-tunnel start --user | stop --user | uninstall --user")
		switch runtime.GOOS {
		case "linux":
			fmt.Println("          systemctl --user status ssh-tunnel-service")
			fmt.Println("  Note:   to start at boot without logging in, enable lingering:")
			fmt.Println("          sudo loginctl enable-linger \"$USER\"")
			fmt.Println("  Ensure ~/.local/bin is on your PATH.")
		case "darwin":
			fmt.Println("          (LaunchAgent in ~/Library/LaunchAgents)")
			fmt.Println("  Ensure ~/.local/bin is on your PATH.")
		}
		return
	}
	fmt.Println("  Manage: ssh-tunnel start | stop | uninstall  (each prompts for elevation)")
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
