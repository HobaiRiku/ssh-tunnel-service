//go:build linux

package service

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"text/template"
)

const (
	openwrtInitScript = "/etc/init.d/" + serviceName
	openwrtBinaryPath = "/usr/sbin/ssh-tunnel"
)

// isOpenWrt reports whether the current system is running OpenWrt, detected
// by the presence of the /etc/openwrt_release marker file.
func isOpenWrt() bool {
	_, err := os.Stat("/etc/openwrt_release")
	return err == nil
}

var openwrtInitTmpl = template.Must(template.New("").Parse(
	`#!/bin/sh /etc/rc.common
USE_PROCD=1
START=95
STOP=01

PROG={{.BinaryPath}}

start_service() {
	procd_open_instance
	procd_set_param command "$PROG"
	procd_set_param env SSH_TUNNEL_HOME={{.Home}}
	procd_set_param stdout 1
	procd_set_param stderr 1
	procd_set_param respawn
	procd_close_instance
}
`))

// openwrtInstall writes the procd init script for the given resolved home and
// enables it (creates the rc.d symlinks) so it starts at boot.
func openwrtInstall(home string) error {
	var buf bytes.Buffer
	if err := openwrtInitTmpl.Execute(&buf, struct {
		BinaryPath string
		Home       string
	}{openwrtBinaryPath, home}); err != nil {
		return fmt.Errorf("render init script: %w", err)
	}
	if err := os.WriteFile(openwrtInitScript, buf.Bytes(), 0o755); err != nil {
		return fmt.Errorf("write %s: %w", openwrtInitScript, err)
	}
	if out, err := exec.Command(openwrtInitScript, "enable").CombinedOutput(); err != nil {
		_ = os.Remove(openwrtInitScript)
		return fmt.Errorf("enable service: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// openwrtUninstall disables the procd service and removes its init script.
func openwrtUninstall() error {
	// Ignore errors from disable — the script may already be absent.
	_, _ = exec.Command(openwrtInitScript, "disable").CombinedOutput()
	if err := os.Remove(openwrtInitScript); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", openwrtInitScript, err)
	}
	return nil
}

// openwrtStart starts the registered procd service.
func openwrtStart() error {
	out, err := exec.Command(openwrtInitScript, "start").CombinedOutput()
	if err != nil {
		return fmt.Errorf("start service: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// openwrtStop stops the running procd service.
func openwrtStop() error {
	out, err := exec.Command(openwrtInitScript, "stop").CombinedOutput()
	if err != nil {
		return fmt.Errorf("stop service: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// openwrtRunService runs the service directly, blocking until SIGTERM or
// SIGINT is received. procd manages the process lifecycle (respawn etc.) so no
// extra supervisor loop is needed here.
func openwrtRunService(home string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return Run(ctx, home, false)
}
