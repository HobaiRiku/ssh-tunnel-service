package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"

	"ssh-tunnel-service/internal/config"
	"ssh-tunnel-service/internal/paths"
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

func (r *Registry) AppConfig() config.AppConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cfg.App
}

func (r *Registry) KeyPath(file string) string {
	if file == "" {
		return ""
	}
	return filepath.Join(r.paths.Keys(), file)
}

// ── Keys ─────────────────────────────────────────────────────────────────────

func (r *Registry) ListKeys() []config.SSHKey {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]config.SSHKey, len(r.cfg.Keys))
	copy(out, r.cfg.Keys)
	return out
}

func (r *Registry) GetKey(id string) (config.SSHKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, key := range r.cfg.Keys {
		if key.ID == id {
			return key, nil
		}
	}
	return config.SSHKey{}, fmt.Errorf("key %q: %w", id, ErrNotFound)
}

func (r *Registry) AddKey(input SSHKeyInput) error {
	r.mu.Lock()
	next := cloneConfig(r.cfg)
	for _, existing := range next.Keys {
		if existing.ID == input.ID {
			r.mu.Unlock()
			return fmt.Errorf("key %q already exists", input.ID)
		}
	}
	key, oldFile, newFilePath, err := r.prepareKey(next, config.SSHKey{}, input)
	if err != nil {
		r.mu.Unlock()
		return err
	}
	next.Keys = append(next.Keys, key)
	if err := r.persist(next); err != nil {
		r.mu.Unlock()
		return err
	}
	r.mu.Unlock()
	return r.finalizeKeyMaterial(oldFile, newFilePath)
}

func (r *Registry) UpdateKey(id string, input SSHKeyInput) error {
	restartIDs := []string{}
	reason := fmt.Sprintf("key %s updated", id)
	r.mu.Lock()
	next := cloneConfig(r.cfg)
	for i, existing := range next.Keys {
		if existing.ID == id {
			key, oldFile, newFilePath, err := r.prepareKey(next, existing, input)
			if err != nil {
				r.mu.Unlock()
				return err
			}
			next.Keys[i] = key
			restartIDs = r.runningTunnelIDsForKeyLocked(id)
			if err := r.persist(next); err != nil {
				r.mu.Unlock()
				return err
			}
			r.mu.Unlock()
			if err := r.finalizeKeyMaterial(oldFile, newFilePath); err != nil {
				return err
			}
			return r.restartRunningTunnels(restartIDs, reason)
		}
	}
	r.mu.Unlock()
	return fmt.Errorf("key %q: %w", id, ErrNotFound)
}

func (r *Registry) DeleteKey(id string) error {
	var key config.SSHKey
	r.mu.Lock()
	next := cloneConfig(r.cfg)
	for _, remote := range next.Remotes {
		if remote.KeyID == id {
			r.mu.Unlock()
			return fmt.Errorf("key %q is referenced by remote %q", id, remote.ID)
		}
	}
	for i, existing := range next.Keys {
		if existing.ID == id {
			key = existing
			next.Keys = append(next.Keys[:i], next.Keys[i+1:]...)
			if err := r.persist(next); err != nil {
				r.mu.Unlock()
				return err
			}
			r.mu.Unlock()
			if key.File != "" {
				if err := os.Remove(r.KeyPath(key.File)); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("remove key file: %w", err)
				}
			}
			return nil
		}
	}
	r.mu.Unlock()
	return fmt.Errorf("key %q: %w", id, ErrNotFound)
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
	return config.Remote{}, fmt.Errorf("remote %q: %w", id, ErrNotFound)
}

func (r *Registry) AddRemote(remote config.Remote) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	next := cloneConfig(r.cfg)
	if err := requireKey(next, remote.KeyID); err != nil {
		return err
	}
	for _, existing := range next.Remotes {
		if existing.ID == remote.ID {
			return fmt.Errorf("remote %q already exists", remote.ID)
		}
	}
	next.Remotes = append(next.Remotes, remote)
	return r.persist(next)
}

func (r *Registry) UpdateRemote(id string, update config.Remote) error {
	restartIDs := []string{}
	reason := fmt.Sprintf("remote %s updated", id)
	r.mu.Lock()
	next := cloneConfig(r.cfg)
	if err := requireKey(next, update.KeyID); err != nil {
		r.mu.Unlock()
		return err
	}
	for i, existing := range next.Remotes {
		if existing.ID == id {
			update.ID = id
			next.Remotes[i] = update
			restartIDs = r.runningTunnelIDsForRemoteLocked(id)
			if err := r.persist(next); err != nil {
				r.mu.Unlock()
				return err
			}
			r.mu.Unlock()
			return r.restartRunningTunnels(restartIDs, reason)
		}
	}
	r.mu.Unlock()
	return fmt.Errorf("remote %q: %w", id, ErrNotFound)
}

func (r *Registry) DeleteRemote(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	next := cloneConfig(r.cfg)
	for _, t := range next.Tunnels {
		if t.RemoteID == id {
			return fmt.Errorf("remote %q is referenced by tunnel %q", id, t.ID)
		}
	}
	for i, existing := range next.Remotes {
		if existing.ID == id {
			next.Remotes = append(next.Remotes[:i], next.Remotes[i+1:]...)
			return r.persist(next)
		}
	}
	return fmt.Errorf("remote %q: %w", id, ErrNotFound)
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
	return TunnelStatus{}, fmt.Errorf("tunnel %q: %w", id, ErrNotFound)
}

func (r *Registry) AddTunnel(t config.Tunnel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	next := cloneConfig(r.cfg)
	if err := requireRemote(next, t.RemoteID); err != nil {
		return err
	}
	for _, existing := range next.Tunnels {
		if existing.ID == t.ID {
			return fmt.Errorf("tunnel %q already exists", t.ID)
		}
	}
	next.Tunnels = append(next.Tunnels, t)
	return r.persist(next)
}

func (r *Registry) UpdateTunnel(id string, update config.Tunnel) error {
	restartIDs := []string{}
	reason := fmt.Sprintf("tunnel %s updated", id)
	r.mu.Lock()
	next := cloneConfig(r.cfg)
	if err := requireRemote(next, update.RemoteID); err != nil {
		r.mu.Unlock()
		return err
	}
	for i, existing := range next.Tunnels {
		if existing.ID == id {
			update.ID = id
			next.Tunnels[i] = update
			if state, _, _ := r.runtime.Get(id); state == StateRunning {
				restartIDs = []string{id}
			}
			if err := r.persist(next); err != nil {
				r.mu.Unlock()
				return err
			}
			r.mu.Unlock()
			return r.restartRunningTunnels(restartIDs, reason)
		}
	}
	r.mu.Unlock()
	return fmt.Errorf("tunnel %q: %w", id, ErrNotFound)
}

func (r *Registry) DeleteTunnel(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if state, _, _ := r.runtime.Get(id); state == StateRunning {
		return fmt.Errorf("tunnel %q is running", id)
	}
	next := cloneConfig(r.cfg)
	for i, existing := range next.Tunnels {
		if existing.ID == id {
			next.Tunnels = append(next.Tunnels[:i], next.Tunnels[i+1:]...)
			return r.persist(next)
		}
	}
	return fmt.Errorf("tunnel %q: %w", id, ErrNotFound)
}

func requireRemote(cfg *config.Config, id string) error {
	for _, remote := range cfg.Remotes {
		if remote.ID == id {
			return nil
		}
	}
	return fmt.Errorf("remote %q: %w", id, ErrNotFound)
}

func requireKey(cfg *config.Config, id string) error {
	if id == "" {
		return nil
	}
	for _, key := range cfg.Keys {
		if key.ID == id {
			return nil
		}
	}
	return fmt.Errorf("key %q: %w", id, ErrNotFound)
}

func (r *Registry) runningTunnelIDsForRemoteLocked(remoteID string) []string {
	ids := make([]string, 0)
	for _, tunnel := range r.cfg.Tunnels {
		if tunnel.RemoteID != remoteID {
			continue
		}
		if state, _, _ := r.runtime.Get(tunnel.ID); state == StateRunning {
			ids = append(ids, tunnel.ID)
		}
	}
	return ids
}

func (r *Registry) runningTunnelIDsForKeyLocked(keyID string) []string {
	ids := make([]string, 0)
	remoteIDs := map[string]bool{}
	for _, remote := range r.cfg.Remotes {
		if remote.KeyID == keyID {
			remoteIDs[remote.ID] = true
		}
	}
	for _, tunnel := range r.cfg.Tunnels {
		if !remoteIDs[tunnel.RemoteID] {
			continue
		}
		if state, _, _ := r.runtime.Get(tunnel.ID); state == StateRunning {
			ids = append(ids, tunnel.ID)
		}
	}
	return ids
}

func (r *Registry) restartRunningTunnels(ids []string, reason string) error {
	if len(ids) == 0 || r.manager == nil {
		return nil
	}
	for _, id := range ids {
		if err := r.manager.Restart(context.Background(), id, reason); err != nil {
			return err
		}
	}
	return nil
}

func (r *Registry) prepareKey(cfg *config.Config, existing config.SSHKey, input SSHKeyInput) (config.SSHKey, string, string, error) {
	id := existing.ID
	if id == "" {
		id = strings.TrimSpace(input.ID)
	}
	if id == "" {
		return config.SSHKey{}, "", "", fmt.Errorf("key id is required")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = existing.Name
	}
	if name == "" {
		return config.SSHKey{}, "", "", fmt.Errorf("key %q: name is required", id)
	}
	description := input.Description
	if input.Description == "" && existing.ID != "" {
		description = existing.Description
	}
	key := config.SSHKey{ID: id, Name: name, Description: description, File: existing.File}

	privateKey := strings.TrimSpace(input.PrivateKey)
	sourcePath := strings.TrimSpace(input.SourcePath)
	if privateKey != "" && sourcePath != "" {
		return config.SSHKey{}, "", "", fmt.Errorf("key %q: provide either private_key or source_path, not both", id)
	}
	if existing.ID == "" && privateKey == "" {
		return config.SSHKey{}, "", "", fmt.Errorf("key %q: private_key is required", id)
	}
	if privateKey == "" {
		return key, "", "", nil
	}

	content := []byte(privateKey)
	if _, err := ssh.ParseRawPrivateKey(content); err != nil {
		return config.SSHKey{}, "", "", fmt.Errorf("key %q: private_key is not a valid SSH private key: %w", id, err)
	}
	fileName, err := pickKeyFileName(id, input.FileName, sourcePath, existing.File)
	if err != nil {
		return config.SSHKey{}, "", "", err
	}
	newFilePath := r.KeyPath(fileName)
	if err := os.WriteFile(newFilePath, content, r.paths.FileMode()); err != nil {
		return config.SSHKey{}, "", "", fmt.Errorf("write key file %s: %w", newFilePath, err)
	}
	key.File = fileName
	oldFile := ""
	if existing.File != "" && existing.File != fileName {
		oldFile = r.KeyPath(existing.File)
	}
	return key, oldFile, newFilePath, nil
}

func (r *Registry) finalizeKeyMaterial(oldFile, newFilePath string) error {
	if newFilePath != "" {
		if err := os.Chmod(newFilePath, r.paths.FileMode()); err != nil {
			return fmt.Errorf("chmod key file %s: %w", newFilePath, err)
		}
	}
	if oldFile != "" {
		if err := os.Remove(oldFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove old key file %s: %w", oldFile, err)
		}
	}
	return nil
}

func pickKeyFileName(id, requested, sourcePath, existing string) (string, error) {
	candidate := strings.TrimSpace(requested)
	if candidate == "" && sourcePath != "" {
		candidate = filepath.Base(sourcePath)
	}
	if candidate == "" {
		candidate = existing
	}
	if candidate == "" {
		candidate = id
	}
	candidate = filepath.Base(strings.TrimSpace(candidate))
	if candidate == "." || candidate == string(filepath.Separator) || candidate == "" {
		return "", fmt.Errorf("key %q: invalid file name", id)
	}
	if filepath.Clean(candidate) != candidate || candidate == ".." {
		return "", fmt.Errorf("key %q: invalid file name", id)
	}
	return candidate, nil
}

// persist writes the current in-memory config back to disk atomically.
// Must be called with r.mu held (write lock).
func (r *Registry) persist(next *config.Config) error {
	if err := config.Validate(next); err != nil {
		return err
	}
	if r.paths.Config() == "" {
		r.cfg = next
		return nil
	}
	if err := config.Write(r.paths.Config(), next, r.paths.FileMode()); err != nil {
		return err
	}
	r.cfg = next
	return nil
}

func cloneConfig(cfg *config.Config) *config.Config {
	if cfg == nil {
		return &config.Config{}
	}
	next := *cfg
	next.Keys = append([]config.SSHKey(nil), cfg.Keys...)
	next.Remotes = append([]config.Remote(nil), cfg.Remotes...)
	next.Tunnels = make([]config.Tunnel, len(cfg.Tunnels))
	for i, tunnel := range cfg.Tunnels {
		next.Tunnels[i] = tunnel
		next.Tunnels[i].SSHOptions = append([]string(nil), tunnel.SSHOptions...)
	}
	return &next
}
