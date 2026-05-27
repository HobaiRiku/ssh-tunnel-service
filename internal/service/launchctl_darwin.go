package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// macOS launchd helpers. kardianos/service still drives `launchctl load -w`,
// which is the legacy API and emits the famously vague "Input/output error"
// whenever launchd has any stale cached state for the same label (common when
// switching between system daemon and user agent installs). These wrappers use
// the modern bootstrap/bootout/kickstart API in the gui/<uid> domain so install
// is self-healing and start/stop don't fail on cache mismatches.

func darwinUserDomain() string {
	return fmt.Sprintf("gui/%d", os.Getuid())
}

func darwinServiceTarget() string {
	return fmt.Sprintf("%s/%s", darwinUserDomain(), serviceName)
}

func darwinPlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", serviceName+".plist"), nil
}

func darwinBootout() {
	// Idempotent best-effort: ignore "service not found" — we just want to
	// guarantee any stale registration is gone.
	_ = exec.Command("launchctl", "bootout", darwinServiceTarget()).Run()
}

func darwinBootstrap() error {
	plist, err := darwinPlistPath()
	if err != nil {
		return fmt.Errorf("locate plist: %w", err)
	}
	out, err := exec.Command("launchctl", "bootstrap", darwinUserDomain(), plist).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl bootstrap: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func darwinKickstart() (bool, error) {
	out, err := exec.Command("launchctl", "kickstart", darwinServiceTarget()).CombinedOutput()
	if err != nil {
		return true, fmt.Errorf("launchctl kickstart: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return true, nil
}

func darwinKill() (bool, error) {
	out, err := exec.Command("launchctl", "kill", "SIGTERM", darwinServiceTarget()).CombinedOutput()
	if err != nil {
		return true, fmt.Errorf("launchctl kill: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return true, nil
}
