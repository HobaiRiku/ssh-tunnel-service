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
	plist, err := darwinPlistPath()
	if err == nil {
		_, _ = runLaunchctl("bootout", darwinUserDomain(), plist)
	}
	_, _ = runLaunchctl("bootout", darwinServiceTarget())
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

func darwinStart() (bool, error) {
	loaded, err := darwinServiceLoaded()
	if err != nil {
		return true, err
	}
	if !loaded {
		return true, darwinBootstrap()
	}
	if _, err := runLaunchctl("kickstart", "-k", darwinServiceTarget()); err != nil {
		return true, err
	}
	return true, nil
}

func darwinStop() (bool, error) {
	plist, err := darwinPlistPath()
	if err != nil {
		return true, fmt.Errorf("locate plist: %w", err)
	}
	if _, err := runLaunchctl("bootout", darwinUserDomain(), plist); err != nil {
		if isLaunchctlNotFound(err) {
			return true, nil
		}
		return true, err
	}
	return true, nil
}

func darwinServiceLoaded() (bool, error) {
	_, err := runLaunchctl("print", darwinServiceTarget())
	if err == nil {
		return true, nil
	}
	if isLaunchctlNotFound(err) {
		return false, nil
	}
	return false, err
}

func runLaunchctl(args ...string) (string, error) {
	out, err := exec.Command("launchctl", args...).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("launchctl %s: %s: %w", strings.Join(args, " "), msg, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func isLaunchctlNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "service not found") ||
		strings.Contains(msg, "Could not find service") ||
		strings.Contains(msg, "Could not find specified service") ||
		strings.Contains(msg, "No such process") ||
		strings.Contains(msg, "Bad request")
}
