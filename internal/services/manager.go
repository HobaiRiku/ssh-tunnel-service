package services

import (
"context"
"fmt"
"log/slog"
"os/exec"
"sync"
)

// Manager launches and tracks ssh child processes for each tunnel.
type Manager struct {
mu      sync.Mutex
procs   map[string]*exec.Cmd
rt      *Runtime
reg     *Registry
logger  *slog.Logger
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

forward := fmt.Sprintf("%s:%d:%s:%d", ts.BindAddress, ts.BindPort, ts.TargetHost, ts.TargetPort)
args := []string{"-N", string(ts.Direction), forward}
args = append(args, ts.SSHOptions...)
args = append(args, "-p", fmt.Sprintf("%d", remote.Port), fmt.Sprintf("%s@%s", remote.User, remote.Host))

m.mu.Lock()
if _, exists := m.procs[id]; exists {
m.mu.Unlock()
return fmt.Errorf("tunnel %q is already running", id)
}
cmd := exec.CommandContext(ctx, "ssh", args...)
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
if waitErr := cmd.Wait(); waitErr != nil {
m.logger.Warn("tunnel exited", "id", id, "err", waitErr)
m.rt.SetError(id, waitErr.Error())
} else {
m.rt.SetStopped(id)
}
m.mu.Lock()
if current, ok := m.procs[id]; ok && current == cmd {
delete(m.procs, id)
}
m.mu.Unlock()
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
