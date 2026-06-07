package services

import (
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
// Every resource is identified by its unique Name; remotes reference a key by
// name and tunnels reference their remote by name.
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
	for i := range out {
		out[i].Public = r.publicKeyFor(out[i].File)
	}
	return out
}

func (r *Registry) GetKey(name string) (config.SSHKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, key := range r.cfg.Keys {
		if key.Name == name {
			key.Public = r.publicKeyFor(key.File)
			return key, nil
		}
	}
	return config.SSHKey{}, fmt.Errorf("key %q: %w", name, ErrNotFound)
}

func (r *Registry) AddKey(input SSHKeyInput) error {
	r.mu.Lock()
	next := cloneConfig(r.cfg)
	key, oldFile, newFilePath, err := r.prepareKey(config.SSHKey{}, input)
	if err != nil {
		r.mu.Unlock()
		return err
	}
	for _, existing := range next.Keys {
		if existing.Name == key.Name {
			r.mu.Unlock()
			return fmt.Errorf("key %q already exists: %w", key.Name, ErrAlreadyExists)
		}
	}
	next.Keys = append(next.Keys, key)
	if err := r.persist(next); err != nil {
		r.mu.Unlock()
		return err
	}
	r.mu.Unlock()
	return r.finalizeKeyMaterial(oldFile, newFilePath)
}

func (r *Registry) UpdateKey(name string, input SSHKeyInput) error {
	r.mu.Lock()
	next := cloneConfig(r.cfg)
	for i, existing := range next.Keys {
		if existing.Name != name {
			continue
		}
		key, oldFile, newFilePath, err := r.prepareKey(existing, input)
		if err != nil {
			r.mu.Unlock()
			return err
		}
		if key.Name != name && containsKeyName(next.Keys, key.Name) {
			r.mu.Unlock()
			return fmt.Errorf("key %q already exists: %w", key.Name, ErrAlreadyExists)
		}
		next.Keys[i] = key
		// A rename must cascade to every remote that references this key.
		if key.Name != name {
			for j := range next.Remotes {
				if next.Remotes[j].Key == name {
					next.Remotes[j].Key = key.Name
				}
			}
		}
		restartNames := r.runningTunnelNamesForKeyLocked(next, key.Name)
		if err := r.persist(next); err != nil {
			r.mu.Unlock()
			return err
		}
		r.mu.Unlock()
		if err := r.finalizeKeyMaterial(oldFile, newFilePath); err != nil {
			return err
		}
		return r.restartRunningTunnels(restartNames, fmt.Sprintf("key %s updated", key.Name))
	}
	r.mu.Unlock()
	return fmt.Errorf("key %q: %w", name, ErrNotFound)
}

func (r *Registry) DeleteKey(name string) error {
	r.mu.Lock()
	next := cloneConfig(r.cfg)
	if next.App.SystemDefaultKey == name {
		r.mu.Unlock()
		return fmt.Errorf("key %q is the system default key; switch app.system_default_key to another key first", name)
	}
	for _, remote := range next.Remotes {
		if remote.Key == name {
			r.mu.Unlock()
			return fmt.Errorf("key %q is referenced by remote %q", name, remote.Name)
		}
	}
	for i, existing := range next.Keys {
		if existing.Name == name {
			key := existing
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
				if err := os.Remove(r.KeyPath(key.File) + pubSuffix); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("remove public key file: %w", err)
				}
			}
			return nil
		}
	}
	r.mu.Unlock()
	return fmt.Errorf("key %q: %w", name, ErrNotFound)
}

// SetSystemDefaultKey designates an existing key as the system default used by
// unbound tunnels under a system service. Switching away from the current
// default is what unlocks deleting it.
func (r *Registry) SetSystemDefaultKey(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !containsKeyName(r.cfg.Keys, name) {
		return fmt.Errorf("key %q: %w", name, ErrNotFound)
	}
	next := cloneConfig(r.cfg)
	next.App.SystemDefaultKey = name
	return r.persist(next)
}

// EnsureSystemDefaultKey guarantees app.system_default_key points at a real key,
// generating a fresh ed25519 key (named systemDefaultKeyName) when it is unset
// or dangling. Idempotent; intended to run once at system-service startup and
// during install provisioning. Returns the designated key name.
func (r *Registry) EnsureSystemDefaultKey() (string, error) {
	r.mu.Lock()
	if name := r.cfg.App.SystemDefaultKey; name != "" && containsKeyName(r.cfg.Keys, name) {
		r.mu.Unlock()
		return name, nil
	}
	r.mu.Unlock()

	// Generate material outside the lock; AddKey re-locks and persists.
	name := r.uniqueDefaultKeyName()
	priv, err := config.GenerateEd25519PrivateKey("ssh-tunnel-service system default")
	if err != nil {
		return "", err
	}
	if err := r.AddKey(SSHKeyInput{
		Name:        name,
		Description: "Auto-generated system default key for unbound tunnels",
		PrivateKey:  priv,
	}); err != nil {
		return "", fmt.Errorf("create system default key: %w", err)
	}
	if err := r.SetSystemDefaultKey(name); err != nil {
		return "", err
	}
	return name, nil
}

const systemDefaultKeyName = "system-default"

// uniqueDefaultKeyName returns systemDefaultKeyName, suffixing it if a key with
// that name already exists (so we never collide with an imported key).
func (r *Registry) uniqueDefaultKeyName() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !containsKeyName(r.cfg.Keys, systemDefaultKeyName) {
		return systemDefaultKeyName
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", systemDefaultKeyName, i)
		if !containsKeyName(r.cfg.Keys, candidate) {
			return candidate
		}
	}
}

// ── Remotes ──────────────────────────────────────────────────────────────────

func (r *Registry) ListRemotes() []config.Remote {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]config.Remote, len(r.cfg.Remotes))
	copy(out, r.cfg.Remotes)
	return out
}

func (r *Registry) GetRemote(name string) (config.Remote, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, remote := range r.cfg.Remotes {
		if remote.Name == name {
			return remote, nil
		}
	}
	return config.Remote{}, fmt.Errorf("remote %q: %w", name, ErrNotFound)
}

func (r *Registry) AddRemote(remote config.Remote) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := config.ValidateName("remote", remote.Name); err != nil {
		return err
	}
	next := cloneConfig(r.cfg)
	if err := requireKey(next, remote.Key); err != nil {
		return err
	}
	if containsRemoteName(next.Remotes, remote.Name) {
		return fmt.Errorf("remote %q already exists: %w", remote.Name, ErrAlreadyExists)
	}
	next.Remotes = append(next.Remotes, remote)
	return r.persist(next)
}

func (r *Registry) UpdateRemote(name string, update config.Remote) error {
	r.mu.Lock()
	if err := config.ValidateName("remote", update.Name); err != nil {
		r.mu.Unlock()
		return err
	}
	next := cloneConfig(r.cfg)
	if err := requireKey(next, update.Key); err != nil {
		r.mu.Unlock()
		return err
	}
	for i, existing := range next.Remotes {
		if existing.Name != name {
			continue
		}
		if update.Name != name && containsRemoteName(next.Remotes, update.Name) {
			r.mu.Unlock()
			return fmt.Errorf("remote %q already exists: %w", update.Name, ErrAlreadyExists)
		}
		next.Remotes[i] = update
		// A rename must cascade to every tunnel that references this remote.
		if update.Name != name {
			for j := range next.Tunnels {
				if next.Tunnels[j].Remote == name {
					next.Tunnels[j].Remote = update.Name
				}
			}
		}
		restartNames := r.runningTunnelNamesForRemoteLocked(next, update.Name)
		if err := r.persist(next); err != nil {
			r.mu.Unlock()
			return err
		}
		r.mu.Unlock()
		return r.restartRunningTunnels(restartNames, fmt.Sprintf("remote %s updated", update.Name))
	}
	r.mu.Unlock()
	return fmt.Errorf("remote %q: %w", name, ErrNotFound)
}

func (r *Registry) DeleteRemote(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	next := cloneConfig(r.cfg)
	for _, t := range next.Tunnels {
		if t.Remote == name {
			return fmt.Errorf("remote %q is referenced by tunnel %q", name, t.Name)
		}
	}
	for i, existing := range next.Remotes {
		if existing.Name == name {
			next.Remotes = append(next.Remotes[:i], next.Remotes[i+1:]...)
			return r.persist(next)
		}
	}
	return fmt.Errorf("remote %q: %w", name, ErrNotFound)
}

// ── Tunnels ───────────────────────────────────────────────────────────────────

func (r *Registry) ListTunnels() []TunnelStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]TunnelStatus, 0, len(r.cfg.Tunnels))
	for _, t := range r.cfg.Tunnels {
		state, pid, errMsg := r.runtime.Get(t.Name)
		out = append(out, TunnelStatus{Tunnel: t, State: state, PID: pid, Error: errMsg})
	}
	return out
}

func (r *Registry) GetTunnel(name string) (TunnelStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, t := range r.cfg.Tunnels {
		if t.Name == name {
			state, pid, errMsg := r.runtime.Get(t.Name)
			return TunnelStatus{Tunnel: t, State: state, PID: pid, Error: errMsg}, nil
		}
	}
	return TunnelStatus{}, fmt.Errorf("tunnel %q: %w", name, ErrNotFound)
}

func (r *Registry) AddTunnel(t config.Tunnel) error {
	r.mu.Lock()
	if err := config.ValidateName("tunnel", t.Name); err != nil {
		r.mu.Unlock()
		return err
	}
	next := cloneConfig(r.cfg)
	if err := requireRemote(next, t.Remote); err != nil {
		r.mu.Unlock()
		return err
	}
	if containsTunnelName(next.Tunnels, t.Name) {
		r.mu.Unlock()
		return fmt.Errorf("tunnel %q already exists: %w", t.Name, ErrAlreadyExists)
	}
	next.Tunnels = append(next.Tunnels, t)
	if err := r.persist(next); err != nil {
		r.mu.Unlock()
		return err
	}
	mgr := r.manager
	r.mu.Unlock()

	// auto_start means "keep it running": start it now (and the manager keeps it
	// alive across drops), not only on the next service restart. A start failure
	// is surfaced through the tunnel's runtime state, not as an add error — the
	// definition was persisted successfully.
	if t.AutoStart && mgr != nil {
		_ = mgr.Start(t.Name)
	}
	return nil
}

func (r *Registry) UpdateTunnel(name string, update config.Tunnel) error {
	r.mu.Lock()
	if err := config.ValidateName("tunnel", update.Name); err != nil {
		r.mu.Unlock()
		return err
	}
	next := cloneConfig(r.cfg)
	if err := requireRemote(next, update.Remote); err != nil {
		r.mu.Unlock()
		return err
	}
	for i, existing := range next.Tunnels {
		if existing.Name != name {
			continue
		}
		if update.Name != name && containsTunnelName(next.Tunnels, update.Name) {
			r.mu.Unlock()
			return fmt.Errorf("tunnel %q already exists: %w", update.Name, ErrAlreadyExists)
		}
		next.Tunnels[i] = update
		state, _, _ := r.runtime.Get(name)
		wasRunning := state == StateRunning
		if err := r.persist(next); err != nil {
			r.mu.Unlock()
			return err
		}
		mgr := r.manager
		r.mu.Unlock()
		return applyTunnelState(mgr, name, update, wasRunning, fmt.Sprintf("tunnel %s updated", update.Name))
	}
	r.mu.Unlock()
	return fmt.Errorf("tunnel %q: %w", name, ErrNotFound)
}

// applyTunnelState reconciles the running ssh process with a tunnel definition
// after an update: a rename moves the process under the new name, a config
// change restarts it, and enabling auto_start brings it up.
func applyTunnelState(mgr *Manager, oldName string, update config.Tunnel, wasRunning bool, reason string) error {
	if mgr == nil {
		return nil
	}
	renamed := update.Name != oldName
	if wasRunning && renamed {
		_ = mgr.Stop(oldName)
		return mgr.Start(update.Name)
	}
	if wasRunning {
		return mgr.Restart(update.Name, reason)
	}
	if update.AutoStart {
		return mgr.Start(update.Name)
	}
	return nil
}

func (r *Registry) DeleteTunnel(name string) error {
	r.mu.Lock()
	if state, _, _ := r.runtime.Get(name); state == StateRunning {
		r.mu.Unlock()
		return fmt.Errorf("tunnel %q is running", name)
	}
	next := cloneConfig(r.cfg)
	for i, existing := range next.Tunnels {
		if existing.Name == name {
			next.Tunnels = append(next.Tunnels[:i], next.Tunnels[i+1:]...)
			if err := r.persist(next); err != nil {
				r.mu.Unlock()
				return err
			}
			mgr := r.manager
			r.mu.Unlock()
			// Cancel any pending reconnect supervision for the removed tunnel.
			if mgr != nil {
				mgr.Forget(name)
			}
			return nil
		}
	}
	r.mu.Unlock()
	return fmt.Errorf("tunnel %q: %w", name, ErrNotFound)
}

func requireRemote(cfg *config.Config, name string) error {
	if containsRemoteName(cfg.Remotes, name) {
		return nil
	}
	return fmt.Errorf("remote %q: %w", name, ErrNotFound)
}

func requireKey(cfg *config.Config, name string) error {
	if name == "" || containsKeyName(cfg.Keys, name) {
		return nil
	}
	return fmt.Errorf("key %q: %w", name, ErrNotFound)
}

func containsKeyName(keys []config.SSHKey, name string) bool {
	for _, k := range keys {
		if k.Name == name {
			return true
		}
	}
	return false
}

func containsRemoteName(remotes []config.Remote, name string) bool {
	for _, r := range remotes {
		if r.Name == name {
			return true
		}
	}
	return false
}

func containsTunnelName(tunnels []config.Tunnel, name string) bool {
	for _, t := range tunnels {
		if t.Name == name {
			return true
		}
	}
	return false
}

func (r *Registry) runningTunnelNamesForRemoteLocked(cfg *config.Config, remoteName string) []string {
	names := make([]string, 0)
	for _, tunnel := range cfg.Tunnels {
		if tunnel.Remote != remoteName {
			continue
		}
		if state, _, _ := r.runtime.Get(tunnel.Name); state == StateRunning {
			names = append(names, tunnel.Name)
		}
	}
	return names
}

func (r *Registry) runningTunnelNamesForKeyLocked(cfg *config.Config, keyName string) []string {
	names := make([]string, 0)
	remoteNames := map[string]bool{}
	for _, remote := range cfg.Remotes {
		if remote.Key == keyName {
			remoteNames[remote.Name] = true
		}
	}
	for _, tunnel := range cfg.Tunnels {
		if !remoteNames[tunnel.Remote] {
			continue
		}
		if state, _, _ := r.runtime.Get(tunnel.Name); state == StateRunning {
			names = append(names, tunnel.Name)
		}
	}
	return names
}

func (r *Registry) restartRunningTunnels(names []string, reason string) error {
	r.mu.RLock()
	mgr := r.manager
	r.mu.RUnlock()
	if len(names) == 0 || mgr == nil {
		return nil
	}
	for _, name := range names {
		if err := mgr.Restart(name, reason); err != nil {
			return err
		}
	}
	return nil
}

func (r *Registry) prepareKey(existing config.SSHKey, input SSHKeyInput) (config.SSHKey, string, string, error) {
	// Validate the raw name (no trimming) so leading/trailing whitespace is
	// rejected, consistent with remotes/tunnels and the UI rules. Fall back to
	// the existing name only when no new name was supplied.
	name := input.Name
	if name == "" {
		name = existing.Name
	}
	if err := config.ValidateName("key", name); err != nil {
		return config.SSHKey{}, "", "", err
	}
	description := input.Description
	if input.Description == "" && existing.Name != "" {
		description = existing.Description
	}
	key := config.SSHKey{Name: name, Description: description, File: existing.File}

	privateKey := strings.TrimSpace(input.PrivateKey)
	sourcePath := strings.TrimSpace(input.SourcePath)
	if privateKey != "" && sourcePath != "" {
		return config.SSHKey{}, "", "", fmt.Errorf("key %q: provide either private_key or source_path, not both", name)
	}
	if existing.Name == "" && privateKey == "" {
		return config.SSHKey{}, "", "", fmt.Errorf("key %q: private_key is required", name)
	}
	if privateKey == "" {
		return key, "", "", nil
	}

	content := []byte(privateKey)
	if _, err := ssh.ParseRawPrivateKey(content); err != nil {
		return config.SSHKey{}, "", "", fmt.Errorf("key %q: private_key is not a valid SSH private key: %w", name, err)
	}
	fileName, err := pickKeyFileName(name, input.FileName, sourcePath, existing.File)
	if err != nil {
		return config.SSHKey{}, "", "", err
	}
	newFilePath := r.KeyPath(fileName)
	if err := os.WriteFile(newFilePath, content, r.paths.FileMode()); err != nil {
		return config.SSHKey{}, "", "", fmt.Errorf("write key file %s: %w", newFilePath, err)
	}
	// Materialize the public key alongside the private one so users can copy it
	// to a server's authorized_keys. Public keys are not secret, but the keys
	// directory is already 0700 so a 0644 file is still only owner-visible.
	pub, err := config.PublicKeyAuthorized(privateKey)
	if err != nil {
		os.Remove(newFilePath)
		return config.SSHKey{}, "", "", fmt.Errorf("key %q: %w", name, err)
	}
	if err := os.WriteFile(newFilePath+pubSuffix, []byte(pub), 0o644); err != nil {
		os.Remove(newFilePath)
		return config.SSHKey{}, "", "", fmt.Errorf("write public key file %s: %w", newFilePath+pubSuffix, err)
	}
	key.File = fileName
	oldFile := ""
	if existing.File != "" && existing.File != fileName {
		oldFile = r.KeyPath(existing.File)
	}
	return key, oldFile, newFilePath, nil
}

// pubSuffix is appended to a managed private key file to name its public key.
const pubSuffix = ".pub"

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
		if err := os.Remove(oldFile + pubSuffix); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove old public key file %s: %w", oldFile+pubSuffix, err)
		}
	}
	return nil
}

// EnsurePublicKeys backfills a `.pub` file next to every managed private key
// that is missing one (e.g. keys created before public-key materialization
// existed). It derives the public key from the private one and writes it 0644.
// Missing or unreadable private keys and derivation failures are skipped so a
// single bad key never blocks startup. Safe to call repeatedly.
func (r *Registry) EnsurePublicKeys() {
	r.mu.RLock()
	files := make([]string, 0, len(r.cfg.Keys))
	for _, key := range r.cfg.Keys {
		if key.File != "" {
			files = append(files, key.File)
		}
	}
	r.mu.RUnlock()

	for _, file := range files {
		privPath := r.KeyPath(file)
		pubPath := privPath + pubSuffix
		if _, err := os.Stat(pubPath); err == nil {
			continue
		}
		priv, err := os.ReadFile(privPath)
		if err != nil {
			continue
		}
		pub, err := config.PublicKeyAuthorized(string(priv))
		if err != nil {
			continue
		}
		_ = os.WriteFile(pubPath, []byte(pub), 0o644)
	}
}

// publicKeyFor returns the stored public key for a managed key file, deriving it
// from the private key as a fallback when the `.pub` is missing (e.g. a key
// imported before public materialization existed). Returns "" on any failure.
func (r *Registry) publicKeyFor(file string) string {
	if file == "" {
		return ""
	}
	privPath := r.KeyPath(file)
	if data, err := os.ReadFile(privPath + pubSuffix); err == nil {
		return strings.TrimSpace(string(data))
	}
	priv, err := os.ReadFile(privPath)
	if err != nil {
		return ""
	}
	pub, err := config.PublicKeyAuthorized(string(priv))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(pub)
}

func pickKeyFileName(name, requested, sourcePath, existing string) (string, error) {
	candidate := strings.TrimSpace(requested)
	if candidate == "" && sourcePath != "" {
		candidate = filepath.Base(sourcePath)
	}
	if candidate == "" {
		candidate = existing
	}
	if candidate == "" {
		candidate = name
	}
	candidate = filepath.Base(strings.TrimSpace(candidate))
	if candidate == "." || candidate == string(filepath.Separator) || candidate == "" {
		return "", fmt.Errorf("key %q: invalid file name", name)
	}
	if filepath.Clean(candidate) != candidate || candidate == ".." {
		return "", fmt.Errorf("key %q: invalid file name", name)
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
