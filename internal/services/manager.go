package services

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/HobaiRiku/ssh-tunnel-service/internal/config"
)

// Manager launches and tracks ssh child processes for each tunnel.
type Manager struct {
	mu     sync.Mutex
	procs  map[string]*exec.Cmd
	rt     *Runtime
	reg    *Registry
	logger *slog.Logger
}

// NewManager creates a tunnel process manager.
func NewManager(reg *Registry, rt *Runtime, logger *slog.Logger) *Manager {
	return &Manager{
		procs:  map[string]*exec.Cmd{},
		rt:     rt,
		reg:    reg,
		logger: logger,
	}
}

// Start launches the tunnel identified by id.
func (m *Manager) Start(ctx context.Context, id string) error {
	ts, err := m.reg.GetTunnel(id)
	if err != nil {
		return err
	}
	remote, err := m.reg.GetRemote(ts.RemoteID)
	if err != nil {
		return err
	}
	appCfg := m.reg.AppConfig()

	forward := fmt.Sprintf("%s:%d:%s:%d", ts.BindAddress, ts.BindPort, ts.TargetHost, ts.TargetPort)
	if appCfg.SSHHostKeyPolicy != config.SSHHostKeyPolicyInsecure {
		if err := ensureKnownHostsFile(appCfg.SSHKnownHosts, 0o600); err != nil {
			m.rt.SetError(id, err.Error())
			return err
		}
	}

	args := []string{"-N", "-o", "BatchMode=yes"}
	args = append(args, sshHostKeyArgs(appCfg)...)
	args = append(args, string(ts.Direction), forward)
	args = append(args, ts.SSHOptions...)
	args = append(args, "-p", fmt.Sprintf("%d", remote.Port), fmt.Sprintf("%s@%s", remote.User, remote.Host))

	m.mu.Lock()
	if _, exists := m.procs[id]; exists {
		m.mu.Unlock()
		return fmt.Errorf("tunnel %q is already running", id)
	}
	cmd := exec.CommandContext(ctx, "ssh", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		m.mu.Unlock()
		m.rt.SetError(id, err.Error())
		return fmt.Errorf("start tunnel %s: %w", id, err)
	}
	m.procs[id] = cmd
	m.rt.SetRunning(id, cmd.Process.Pid)
	m.mu.Unlock()

	m.logger.Info("tunnel started", "id", id, "pid", cmd.Process.Pid, "args", args)
	go func() {
		waitErr := cmd.Wait()
		stderrText := strings.TrimSpace(stderr.String())

		m.mu.Lock()
		current, ok := m.procs[id]
		if !ok || current != cmd {
			m.mu.Unlock()
			return
		}
		delete(m.procs, id)
		m.mu.Unlock()

		if waitErr != nil {
			if ctx.Err() == nil {
				diagnostic := diagnoseSSHFailure(stderrText)
				m.logger.Warn("tunnel exited", "id", id, "err", waitErr, "stderr", stderrText, "diagnostic", diagnostic)
				m.rt.SetError(id, diagnostic)
				return
			}
		}
		m.rt.SetStopped(id)
	}()
	return nil
}

// Stop kills the running tunnel process.
func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cmd, ok := m.procs[id]
	if !ok {
		return fmt.Errorf("tunnel %q is not running", id)
	}
	if cmd.Process != nil {
		if err := cmd.Process.Kill(); err != nil {
			return fmt.Errorf("kill tunnel %s: %w", id, err)
		}
		delete(m.procs, id)
	}
	m.rt.SetStopped(id)
	m.logger.Info("tunnel stopped", "id", id)
	return nil
}

// AutoStart launches all tunnels with auto_start: true.
func (m *Manager) AutoStart(ctx context.Context) {
	for _, ts := range m.reg.ListTunnels() {
		if ts.AutoStart {
			if err := m.Start(ctx, ts.ID); err != nil {
				m.logger.Error("autostart tunnel", "id", ts.ID, "err", err)
			}
		}
	}
}

func sshHostKeyArgs(appCfg config.AppConfig) []string {
	switch appCfg.SSHHostKeyPolicy {
	case config.SSHHostKeyPolicyStrict:
		return []string{
			"-o", "StrictHostKeyChecking=yes",
			"-o", "UserKnownHostsFile=" + appCfg.SSHKnownHosts,
		}
	case config.SSHHostKeyPolicyInsecure:
		return []string{
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=" + os.DevNull,
		}
	default:
		return []string{
			"-o", "StrictHostKeyChecking=accept-new",
			"-o", "UserKnownHostsFile=" + appCfg.SSHKnownHosts,
		}
	}
}

func ensureKnownHostsFile(path string, mode os.FileMode) error {
	if path == "" {
		return fmt.Errorf("ssh known_hosts file path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("prepare known_hosts dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, mode)
	if err != nil {
		return fmt.Errorf("prepare known_hosts file: %w", err)
	}
	if closeErr := f.Close(); closeErr != nil {
		return fmt.Errorf("close known_hosts file: %w", closeErr)
	}
	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf("chmod known_hosts file: %w", err)
	}
	return nil
}

func diagnoseSSHFailure(stderr string) string {
	if stderr == "" {
		return "ssh exited without diagnostic output"
	}
	lower := strings.ToLower(stderr)
	switch {
	case strings.Contains(lower, "host key verification failed"):
		return "host key verification failed; trust the remote host key or adjust app.ssh_host_key_policy"
	case strings.Contains(lower, "remote host identification has changed"),
		strings.Contains(lower, "offending"),
		strings.Contains(lower, "man-in-the-middle"):
		return "remote host key changed; inspect the server and refresh the configured known_hosts file"
	case strings.Contains(lower, "permission denied"):
		return "ssh authentication failed; verify keys, agent access, and remote user permissions"
	case strings.Contains(lower, "could not resolve hostname"),
		strings.Contains(lower, "name or service not known"):
		return "ssh remote hostname could not be resolved; verify the remote host setting"
	case strings.Contains(lower, "connection refused"):
		return "ssh connection refused; verify the remote host, port, and sshd availability"
	case strings.Contains(lower, "connection timed out"),
		strings.Contains(lower, "operation timed out"),
		strings.Contains(lower, "no route to host"):
		return "ssh network connection failed; verify reachability to the remote host"
	default:
		return stderr
	}
}
