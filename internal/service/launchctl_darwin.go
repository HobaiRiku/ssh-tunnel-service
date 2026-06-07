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
// whenever launchd has any stale cached state for the same label. These wrappers
// use the modern bootstrap/bootout/kickstart API so install is self-healing and
// start/stop don't fail on cache mismatches.
//
// Two scopes are supported:
//
//   - system (default): a LaunchDaemon in /Library/LaunchDaemons, managed in the
//     `system` domain, starting at boot without an interactive login. Requires
//     root, which the elevated control commands guarantee.
//   - user (--user): a LaunchAgent in ~/Library/LaunchAgents, managed in the
//     `gui/<uid>` domain. Runs as the logged-in user, no root required.

func darwinDomain(user bool) string {
	if user {
		return fmt.Sprintf("gui/%d", os.Getuid())
	}
	return "system"
}

func darwinServiceTarget(user bool) string {
	return fmt.Sprintf("%s/%s", darwinDomain(user), serviceName)
}

func darwinPlistPath(user bool) (string, error) {
	if user {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return "", fmt.Errorf("resolve user home for LaunchAgent: %w", err)
		}
		return filepath.Join(home, "Library", "LaunchAgents", serviceName+".plist"), nil
	}
	return filepath.Join("/Library/LaunchDaemons", serviceName+".plist"), nil
}

func darwinBootout(user bool) {
	plist, err := darwinPlistPath(user)
	if err == nil {
		_, _ = runLaunchctl("bootout", darwinDomain(user), plist)
	}
	_, _ = runLaunchctl("bootout", darwinServiceTarget(user))
}

func darwinBootstrap(user bool) error {
	plist, err := darwinPlistPath(user)
	if err != nil {
		return fmt.Errorf("locate plist: %w", err)
	}
	out, err := exec.Command("launchctl", "bootstrap", darwinDomain(user), plist).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl bootstrap: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func darwinStart(user bool) (bool, error) {
	loaded, err := darwinServiceLoaded(user)
	if err != nil {
		return true, err
	}
	if !loaded {
		return true, darwinBootstrap(user)
	}
	if _, err := runLaunchctl("kickstart", "-k", darwinServiceTarget(user)); err != nil {
		return true, err
	}
	return true, nil
}

func darwinStop(user bool) (bool, error) {
	plist, err := darwinPlistPath(user)
	if err != nil {
		return true, fmt.Errorf("locate plist: %w", err)
	}
	if _, err := runLaunchctl("bootout", darwinDomain(user), plist); err != nil {
		if isLaunchctlNotFound(err) {
			return true, nil
		}
		return true, err
	}
	return true, nil
}

func darwinServiceLoaded(user bool) (bool, error) {
	_, err := runLaunchctl("print", darwinServiceTarget(user))
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
