package cmd

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"ssh-tunnel-service/internal/service"
	"ssh-tunnel-service/internal/version"
)

func updateCmd() *cobra.Command {
	var (
		userScope bool
		assumeYes bool
	)
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Refresh the installed service binary from this one and restart",
		Long: "Make an upgraded binary take effect.\n\n" +
			"`install` copies the binary to a stable path and points the service unit at\n" +
			"that copy, so upgrading through a package manager replaces the package's own\n" +
			"binary while the service keeps running the old copy. This command copies the\n" +
			"binary you invoke over the installed one and restarts the service.\n\n" +
			"Run it with the upgraded binary — if that is not first on your PATH, call it\n" +
			"by its full path (for example /opt/homebrew/bin/ssh-tunnel update).",
		RunE: func(c *cobra.Command, _ []string) error {
			return runUpdate(c, userScope, assumeYes)
		},
	}
	cmd.Flags().BoolVar(&userScope, "user", false, "update the per-user service")
	cmd.Flags().BoolVarP(&assumeYes, "yes", "y", false, "do not ask for confirmation")
	return cmd
}

func runUpdate(c *cobra.Command, userScope, assumeYes bool) error {
	dest, err := service.BinaryPath(userScope)
	if err != nil {
		return err
	}
	if !service.Installed(userScope) {
		return fmt.Errorf("no %s service is installed at %s; run `ssh-tunnel install%s` first",
			scopeWord(userScope), dest, userFlagSuffix(userScope))
	}
	src, err := currentExecutable()
	if err != nil {
		return err
	}

	// The package prefix and the install destination coincide on some platforms
	// (Homebrew on Intel macOS installs into /usr/local/bin, which is also where
	// the system service copy lives). There the upgrade already landed on the
	// installed binary and only the restart is outstanding.
	inPlace := sameFile(src, dest)
	running := runningInstance()

	out := c.OutOrStdout()
	fmt.Fprintf(out, "  This binary:  %-20s %s\n", version.Version, src)
	if running == nil {
		fmt.Fprintf(out, "  Service:      %-20s %s  (not running)\n", "?", dest)
	} else {
		fmt.Fprintf(out, "  Service:      %-20s %s  (running)\n", running.Version, dest)
	}

	if inPlace && running != nil && running.Version == version.Version {
		fmt.Fprintln(out, "\nAlready up to date; nothing to do.")
		return nil
	}
	if inPlace {
		fmt.Fprintln(out, "\nThe installed binary is already this one; only a restart is needed.")
	}

	if !assumeYes {
		if err := confirmUpdate(out, dest, running != nil); err != nil {
			return err
		}
	}

	// Elevation must come after the prompt: the macOS authentication dialog
	// gives the child no TTY to ask on, so the answer is carried across as --yes.
	if !userScope {
		if handled, err := ensurePrivileged("--yes"); handled {
			return err
		}
	}

	if !inPlace {
		written, replaced, err := service.RefreshBinary(userScope)
		if err != nil {
			return err
		}
		if replaced {
			fmt.Fprintf(out, "  refreshed     %s  %s\n", written, version.Version)
		}
	}

	// Stop tolerates an already-stopped service: the point is to end up running
	// the new binary, not to assert what it was doing before.
	if err := service.Stop(rootFlags.Home, userScope); err != nil {
		fmt.Fprintf(os.Stderr, "  note: stop reported %v\n", err)
	}
	if err := service.Start(rootFlags.Home, userScope); err != nil {
		return fmt.Errorf("restart %s service: %w", scopeWord(userScope), err)
	}
	fmt.Fprintf(out, "  restarted     %s service\n", scopeWord(userScope))
	fmt.Fprintln(out, "\nAuto-start tunnels come back on their own; check with `ssh-tunnel tunnel list`.")
	return nil
}

var errNeedsYes = fmt.Errorf("refusing to restart the service without confirmation; re-run with --yes")

// confirmUpdate asks before a restart that drops live forwards, naming how many
// are running so the cost is explicit rather than implied.
func confirmUpdate(out io.Writer, dest string, serviceRunning bool) error {
	fmt.Fprintf(out, "\nThis will replace %s and restart the service.\n", dest)
	if serviceRunning {
		if n := runningTunnelCount(); n > 0 {
			fmt.Fprintf(out, "%d running tunnel(s) will drop and reconnect.\n", n)
		}
	}

	if !stdinIsTTY() {
		return errNeedsYes
	}
	fmt.Fprint(out, "\nContinue? [y/N] ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	// stdinIsTTY only tests for a character device, which /dev/null also is, so
	// an immediate EOF is the reliable signal that nobody is there to answer.
	if err != nil && strings.TrimSpace(line) == "" {
		return errNeedsYes
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	default:
		return fmt.Errorf("aborted")
	}
}

// runningInstance reports the attached instance, or nil when nothing answers.
// Every failure is "not running" — this is advisory output, not a gate.
func runningInstance() *instancePayload {
	defer restoreBanner(bannerSuppressed)
	bannerSuppressed = true
	client, err := resolveClient(rootFlags.Home)
	if err != nil {
		return nil
	}
	var info instancePayload
	if err := client.request(http.MethodGet, "/api/instance", nil, &info); err != nil {
		return nil
	}
	return &info
}

func runningTunnelCount() int {
	defer restoreBanner(bannerSuppressed)
	bannerSuppressed = true
	client, err := resolveClient(rootFlags.Home)
	if err != nil {
		return 0
	}
	var tunnels []struct {
		State string `json:"state"`
	}
	if err := client.request(http.MethodGet, "/api/tunnels", nil, &tunnels); err != nil {
		return 0
	}
	n := 0
	for _, t := range tunnels {
		if t.State == "running" {
			n++
		}
	}
	return n
}

func restoreBanner(prev bool) { bannerSuppressed = prev }

func currentExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve current executable: %w", err)
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return exe, nil
}

// sameFile compares two paths through their symlinks, falling back to a string
// comparison when either side cannot be resolved.
func sameFile(a, b string) bool {
	ra, err := filepath.EvalSymlinks(a)
	if err != nil {
		ra = a
	}
	rb, err := filepath.EvalSymlinks(b)
	if err != nil {
		rb = b
	}
	return filepath.Clean(ra) == filepath.Clean(rb)
}

func scopeWord(user bool) string {
	if user {
		return "user"
	}
	return "system"
}

func userFlagSuffix(user bool) string {
	if user {
		return " --user"
	}
	return ""
}
