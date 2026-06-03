//go:build darwin

package elevate

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// relaunch re-executes the binary as root. With a terminal we use sudo (inline
// password prompt). Without one — e.g. launched from a .app bundle or Finder —
// we fall back to osascript, which raises the native macOS administrator
// authentication dialog.
func relaunch(extraArgs []string) error {
	if hasTTY() {
		return sudoRelaunch(extraArgs)
	}
	return osascriptRelaunch(extraArgs)
}

func osascriptRelaunch(extraArgs []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}
	// Build a properly quoted shell command for AppleScript's `do shell script`.
	parts := append([]string{exe}, os.Args[1:]...)
	parts = append(parts, extraArgs...)
	quoted := make([]string, len(parts))
	for i, p := range parts {
		quoted[i] = shellQuote(p)
	}
	shellCmd := strings.Join(quoted, " ")
	script := fmt.Sprintf(`do shell script %s with administrator privileges`, appleScriptQuote(shellCmd))
	cmd := exec.Command("osascript", "-e", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "User canceled") || strings.Contains(msg, "-128") {
			return ErrDeclined
		}
		if msg != "" {
			return fmt.Errorf("osascript elevation failed: %s", msg)
		}
		return fmt.Errorf("osascript elevation failed: %w", err)
	}
	return nil
}

// shellQuote wraps s in single quotes, escaping embedded single quotes, so it
// survives the shell that `do shell script` spawns.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// appleScriptQuote wraps s in double quotes for an AppleScript string literal.
func appleScriptQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
