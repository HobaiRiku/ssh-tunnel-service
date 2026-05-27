package services

import (
"fmt"
"sync"

"github.com/HobaiRiku/ssh-tunnel-service/internal/config"
"github.com/HobaiRiku/ssh-tunnel-service/internal/paths"
)

// Registry wires together config, remotes, tunnels, runtime, and the manager.
type Registry struct {
mu      sync.RWMutex
cfg     *config.Config
paths   paths.Paths
runtime *Runtime
manager *Manager
}

// New initializes a Registry. manager may be nil (set via SetManager later).
func New(cfg *config.Config, p paths.Paths, rt *Runtime) *Registry {
return &Registry{cfg: cfg, paths: p, runtime: rt}
}

func (r *Registry) SetManager(m *Manager) {
r.mu.Lock()
defer r.mu.Unlock()
r.manager = m
}

// ── Remotes ──────────────────────────────────────────────────────────────────

func (r *Registry) ListRemotes() []config.Remote {
r.mu.RLock()
defer r.mu.RUnlock()
out := make([]config.Remote, len(r.cfg.Remotes))
copy(out, r.cfg.Remotes)
return out
}

func (r *Registry) GetRemote(id string) (config.Remote, error) {
r.mu.RLock()
defer r.mu.RUnlock()
for _, remote := range r.cfg.Remotes {
if remote.ID == id {
return remote, nil
}
}
return config.Remote{}, fmt.Errorf("remote %q not found", id)
}

func (r *Registry) AddRemote(remote config.Remote) error {
r.mu.Lock()
defer r.mu.Unlock()
for _, existing := range r.cfg.Remotes {
if existing.ID == remote.ID {
return fmt.Errorf("remote %q already exists", remote.ID)
}
}
r.cfg.Remotes = append(r.cfg.Remotes, remote)
return r.persist()
}

func (r *Registry) UpdateRemote(id string, update config.Remote) error {
r.mu.Lock()
defer r.mu.Unlock()
for i, existing := range r.cfg.Remotes {
if existing.ID == id {
update.ID = id
r.cfg.Remotes[i] = update
return r.persist()
}
}
return fmt.Errorf("remote %q not found", id)
}

func (r *Registry) DeleteRemote(id string) error {
r.mu.Lock()
defer r.mu.Unlock()
for _, t := range r.cfg.Tunnels {
if t.RemoteID == id {
return fmt.Errorf("remote %q is referenced by tunnel %q", id, t.ID)
}
}
for i, existing := range r.cfg.Remotes {
if existing.ID == id {
r.cfg.Remotes = append(r.cfg.Remotes[:i], r.cfg.Remotes[i+1:]...)
return r.persist()
}
}
return fmt.Errorf("remote %q not found", id)
}

// ── Tunnels ───────────────────────────────────────────────────────────────────

func (r *Registry) ListTunnels() []TunnelStatus {
r.mu.RLock()
defer r.mu.RUnlock()
out := make([]TunnelStatus, 0, len(r.cfg.Tunnels))
for _, t := range r.cfg.Tunnels {
state, pid, errMsg := r.runtime.Get(t.ID)
out = append(out, TunnelStatus{Tunnel: t, State: state, PID: pid, Error: errMsg})
}
return out
}

func (r *Registry) GetTunnel(id string) (TunnelStatus, error) {
r.mu.RLock()
defer r.mu.RUnlock()
for _, t := range r.cfg.Tunnels {
if t.ID == id {
state, pid, errMsg := r.runtime.Get(t.ID)
return TunnelStatus{Tunnel: t, State: state, PID: pid, Error: errMsg}, nil
}
}
return TunnelStatus{}, fmt.Errorf("tunnel %q not found", id)
}

func (r *Registry) AddTunnel(t config.Tunnel) error {
r.mu.Lock()
defer r.mu.Unlock()
if err := r.requireRemote(t.RemoteID); err != nil {
return err
}
for _, existing := range r.cfg.Tunnels {
if existing.ID == t.ID {
return fmt.Errorf("tunnel %q already exists", t.ID)
}
}
r.cfg.Tunnels = append(r.cfg.Tunnels, t)
return r.persist()
}

func (r *Registry) UpdateTunnel(id string, update config.Tunnel) error {
r.mu.Lock()
defer r.mu.Unlock()
if err := r.requireRemote(update.RemoteID); err != nil {
return err
}
for i, existing := range r.cfg.Tunnels {
if existing.ID == id {
update.ID = id
r.cfg.Tunnels[i] = update
return r.persist()
}
}
return fmt.Errorf("tunnel %q not found", id)
}

func (r *Registry) DeleteTunnel(id string) error {
r.mu.Lock()
defer r.mu.Unlock()
for i, existing := range r.cfg.Tunnels {
if existing.ID == id {
r.cfg.Tunnels = append(r.cfg.Tunnels[:i], r.cfg.Tunnels[i+1:]...)
return r.persist()
}
}
return fmt.Errorf("tunnel %q not found", id)
}

func (r *Registry) requireRemote(id string) error {
for _, remote := range r.cfg.Remotes {
if remote.ID == id {
return nil
}
}
return fmt.Errorf("remote %q not found", id)
}

// persist writes the current in-memory config back to disk atomically.
// Must be called with r.mu held (write lock).
func (r *Registry) persist() error {
if r.paths.Config() == "" {
return nil
}
return config.Write(r.paths.Config(), r.cfg, r.paths.FileMode())
}
