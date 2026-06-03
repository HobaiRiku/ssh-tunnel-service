package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"ssh-tunnel-service/internal/config"
)

// Reconnect backoff bounds for supervised (auto_start) tunnels.
const (
	reconnectBaseDelay   = 2 * time.Second
	reconnectMaxDelay    = 60 * time.Second
	reconnectStableAfter = 20 * time.Second // uptime past which backoff resets
)

// stopGracePeriod is how long Stop waits for ssh to exit after a graceful
// terminate signal before escalating to SIGKILL.
const stopGracePeriod = 5 * time.Second

// managedProc tracks a running ssh child together with a channel that is closed
// once the process has exited and been reaped, so Stop/Restart can wait for the
// old connection to be fully gone before re-binding ports.
type managedProc struct {
	cmd  *exec.Cmd
	done chan struct{}
}

// Manager launches and tracks ssh child processes for each tunnel (keyed by
// tunnel name). auto_start tunnels are supervised: if the ssh process exits
// unexpectedly the manager reconnects with exponential backoff until the tunnel
// is stopped or removed.
type Manager struct {
	mu       sync.Mutex
	procs    map[string]*managedProc
	desired  map[string]bool        // name -> should be running (supervised)
	attempts map[string]int         // name -> consecutive reconnect attempts
	timers   map[string]*time.Timer // name -> pending reconnect timer
	baseCtx  context.Context
	rt       *Runtime
	reg      *Registry
	logger   *slog.Logger
	// systemService is true when running as a system service. It selects the
	// key-injection policy for tunnels whose remote binds no explicit key:
	// system services fall back to app.system_default_key, while per-user
	// (session) runs pass no -i and let ssh use its own default identities.
	systemService bool
}

// NewManager creates a tunnel process manager. baseCtx bounds the lifetime of
// every spawned ssh process; when it is cancelled (service shutdown) the
// children are killed and supervision stops. systemService selects the unbound
// tunnel key policy (see Manager.systemService).
func NewManager(baseCtx context.Context, reg *Registry, rt *Runtime, logger *slog.Logger, systemService bool) *Manager {
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	return &Manager{
		procs:         map[string]*managedProc{},
		desired:       map[string]bool{},
		attempts:      map[string]int{},
		timers:        map[string]*time.Timer{},
		baseCtx:       baseCtx,
		rt:            rt,
		reg:           reg,
		logger:        logger,
		systemService: systemService,
	}
}

// Start launches the tunnel identified by name and marks it as desired so that,
// if it is an auto_start tunnel, the manager keeps it alive.
func (m *Manager) Start(name string) error {
	ts, remote, key, appCfg, err := m.lookupTunnel(name)
	if err != nil {
		return err
	}
	if appCfg.SSHHostKeyPolicy != config.SSHHostKeyPolicyInsecure {
		if err := ensureKnownHostsFile(appCfg.SSHKnownHosts, 0o600); err != nil {
			m.rt.SetError(name, err.Error())
			return err
		}
	}
	if key != nil {
		if err := ensurePrivateKeyFile(m.reg.KeyPath(key.File), 0o600); err != nil {
			m.rt.SetError(name, err.Error())
			return err
		}
	}

	args := sshArgs(ts.Tunnel, remote, key, appCfg, m.reg)

	m.mu.Lock()
	if _, exists := m.procs[name]; exists {
		m.mu.Unlock()
		return fmt.Errorf("tunnel %q is already running", name)
	}
	m.cancelTimerLocked(name)
	cmd := exec.CommandContext(m.baseCtx, "ssh", args...)
	// On baseCtx cancellation (service shutdown) ask ssh to exit cleanly rather
	// than the default SIGKILL, so it releases any -R remote-forward listener on
	// the server before going away; WaitDelay then escalates to SIGKILL if it
	// overstays. This keeps shutdown from orphaning ssh processes that hold a
	// forwarded port and block the next start.
	cmd.Cancel = func() error { return signalTerminate(cmd.Process) }
	cmd.WaitDelay = stopGracePeriod
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		m.mu.Unlock()
		m.rt.SetError(name, err.Error())
		return fmt.Errorf("start tunnel %s: %w", name, err)
	}
	mp := &managedProc{cmd: cmd, done: make(chan struct{})}
	m.procs[name] = mp
	m.desired[name] = true
	m.rt.SetRunning(name, cmd.Process.Pid)
	startedAt := time.Now()
	m.mu.Unlock()

	m.logger.Info("tunnel started", "name", name, "pid", cmd.Process.Pid, "remote", remote.Name, "key", remote.Key, "args", args)
	go m.waitProcess(name, mp, &stderr, startedAt)
	return nil
}

func (m *Manager) waitProcess(name string, mp *managedProc, stderr *bytes.Buffer, startedAt time.Time) {
	defer close(mp.done)
	waitErr := mp.cmd.Wait()
	stderrText := strings.TrimSpace(stderr.String())

	m.mu.Lock()
	current, ok := m.procs[name]
	if !ok || current != mp {
		m.mu.Unlock()
		return
	}
	delete(m.procs, name)
	shuttingDown := m.baseCtx.Err() != nil
	intentional := !m.desired[name]
	if time.Since(startedAt) >= reconnectStableAfter {
		m.attempts[name] = 0
	}
	reconnect := !intentional && !shuttingDown
	m.mu.Unlock()

	if intentional || shuttingDown {
		m.rt.SetStopped(name)
		return
	}

	if waitErr != nil {
		diagnostic := diagnoseSSHFailure(stderrText)
		m.logger.Warn("tunnel exited", "name", name, "err", waitErr, "stderr", stderrText, "diagnostic", diagnostic)
		m.rt.SetError(name, diagnostic)
	} else {
		m.rt.SetStopped(name)
	}

	if reconnect {
		m.scheduleReconnect(name)
	}
}

// scheduleReconnect arms a backoff timer to bring an auto_start tunnel back up.
func (m *Manager) scheduleReconnect(name string) {
	ts, err := m.reg.GetTunnel(name)
	if err != nil || !ts.AutoStart {
		return
	}
	m.mu.Lock()
	if !m.desired[name] || m.baseCtx.Err() != nil {
		m.mu.Unlock()
		return
	}
	m.attempts[name]++
	delay := reconnectBaseDelay << (m.attempts[name] - 1)
	if delay > reconnectMaxDelay || delay <= 0 {
		delay = reconnectMaxDelay
	}
	m.cancelTimerLocked(name)
	m.timers[name] = time.AfterFunc(delay, func() { m.reconnect(name) })
	m.mu.Unlock()
	m.logger.Info("tunnel reconnect scheduled", "name", name, "delay", delay.String())
}

func (m *Manager) reconnect(name string) {
	m.mu.Lock()
	delete(m.timers, name)
	stop := !m.desired[name] || m.baseCtx.Err() != nil
	_, running := m.procs[name]
	m.mu.Unlock()
	if stop || running {
		return
	}
	if ts, err := m.reg.GetTunnel(name); err != nil || !ts.AutoStart {
		return
	}
	if err := m.Start(name); err != nil {
		m.logger.Warn("tunnel reconnect failed", "name", name, "err", err)
		m.scheduleReconnect(name)
	}
}

// Command returns an ssh command equivalent to what the service launches for
// name, but with the identity (-i) deliberately omitted. The preview is meant to
// be copy-pasted into an interactive session where ssh selects the key from the
// agent / ~/.ssh defaults; the managed key the service actually injects (an
// explicit remote key or the system default) is an implementation detail of the
// non-interactive service run and would not exist in the user's session.
func (m *Manager) Command(name string) (TunnelCommandPreview, error) {
	ts, remote, _, appCfg, err := m.lookupTunnel(name)
	if err != nil {
		return TunnelCommandPreview{}, err
	}
	args := sshArgs(ts.Tunnel, remote, nil, appCfg, m.reg)
	return TunnelCommandPreview{
		Command: shellCommand("ssh", args),
		Args:    args,
	}, nil
}

// Stop terminates the running tunnel process and cancels supervision/reconnects.
// It signals ssh to shut down gracefully and waits for it to exit before
// returning. The wait is what makes a subsequent Start (e.g. Restart) reliable:
// a -R remote forward keeps the server-side listening port bound until the ssh
// connection is gone, so re-binding it (ExitOnForwardFailure=yes) only succeeds
// once the old process has fully exited.
func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	m.desired[name] = false
	hadTimer := m.timers[name] != nil
	m.cancelTimerLocked(name)
	mp, running := m.procs[name]
	m.mu.Unlock()

	if running && mp.cmd.Process != nil {
		m.terminate(name, mp)
	}
	if !running && !hadTimer {
		return fmt.Errorf("tunnel %q is %w", name, ErrNotRunning)
	}
	m.rt.SetStopped(name)
	m.logger.Info("tunnel stopped", "name", name)
	return nil
}

// terminate asks ssh to exit cleanly (SIGTERM) so the server releases any -R
// remote-forward listener immediately, then waits for the process to actually
// exit — escalating to SIGKILL if it overstays the grace period.
func (m *Manager) terminate(name string, mp *managedProc) {
	if err := signalTerminate(mp.cmd.Process); err != nil {
		_ = mp.cmd.Process.Kill()
		<-mp.done
		return
	}
	select {
	case <-mp.done:
	case <-time.After(stopGracePeriod):
		m.logger.Warn("tunnel did not exit gracefully; killing", "name", name)
		_ = mp.cmd.Process.Kill()
		<-mp.done
	}
}

// Forget drops all supervision state for a removed tunnel.
func (m *Manager) Forget(name string) {
	m.mu.Lock()
	m.cancelTimerLocked(name)
	delete(m.desired, name)
	delete(m.attempts, name)
	m.mu.Unlock()
}

func (m *Manager) cancelTimerLocked(name string) {
	if t := m.timers[name]; t != nil {
		t.Stop()
		delete(m.timers, name)
	}
}

// Restart brings a tunnel back up cleanly — to apply configuration changes or
// to recover a stale connection. Stop waits for any running ssh to exit so the
// (remote) forward ports are released before Start re-binds them; a tunnel that
// is already stopped (or exits between calls) is simply started, so a not-running
// Stop is not an error.
func (m *Manager) Restart(name, reason string) error {
	m.logger.Info("restarting tunnel", "name", name, "reason", reason)
	if err := m.Stop(name); err != nil && !errors.Is(err, ErrNotRunning) {
		return err
	}
	if err := m.Start(name); err != nil {
		m.logger.Error("tunnel restart failed", "name", name, "reason", reason, "err", err)
		return err
	}
	m.logger.Info("tunnel restarted", "name", name, "reason", reason)
	return nil
}

// IsRunning reports whether the tunnel currently has a managed process.
func (m *Manager) IsRunning(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.procs[name]
	return ok
}

// AutoStart launches all tunnels with auto_start: true.
func (m *Manager) AutoStart() {
	for _, ts := range m.reg.ListTunnels() {
		if ts.AutoStart {
			if err := m.Start(ts.Name); err != nil {
				m.logger.Error("autostart tunnel", "name", ts.Name, "err", err)
			}
		}
	}
}

// Shutdown stops supervising every tunnel and gracefully terminates all running
// ssh processes, blocking until they have exited. The service calls this on
// shutdown so no orphaned ssh process survives holding a forwarded port — which
// would otherwise make the next start fail (notably for -R remote forwards,
// whose server-side listener lingers until the ssh connection is gone).
func (m *Manager) Shutdown() {
	m.mu.Lock()
	for name := range m.desired {
		m.desired[name] = false
	}
	for name := range m.timers {
		m.cancelTimerLocked(name)
	}
	running := make(map[string]*managedProc, len(m.procs))
	for name, mp := range m.procs {
		running[name] = mp
	}
	m.mu.Unlock()

	var wg sync.WaitGroup
	for name, mp := range running {
		if mp.cmd.Process == nil {
			continue
		}
		wg.Add(1)
		go func(name string, mp *managedProc) {
			defer wg.Done()
			m.terminate(name, mp)
			m.rt.SetStopped(name)
		}(name, mp)
	}
	wg.Wait()
	m.logger.Info("all tunnels stopped for shutdown")
}

func (m *Manager) lookupTunnel(name string) (TunnelStatus, config.Remote, *config.SSHKey, config.AppConfig, error) {
	ts, err := m.reg.GetTunnel(name)
	if err != nil {
		return TunnelStatus{}, config.Remote{}, nil, config.AppConfig{}, err
	}
	remote, err := m.reg.GetRemote(ts.Remote)
	if err != nil {
		return TunnelStatus{}, config.Remote{}, nil, config.AppConfig{}, err
	}
	appCfg := m.reg.AppConfig()
	// An explicit remote key wins. Otherwise, under a system service, fall back
	// to the managed system default key; in a session run we leave key nil so
	// ssh uses its own default identities (agent / ~/.ssh/id_*).
	keyName := remote.Key
	if keyName == "" && m.systemService {
		keyName = appCfg.SystemDefaultKey
	}
	var key *config.SSHKey
	if keyName != "" {
		resolvedKey, err := m.reg.GetKey(keyName)
		if err != nil {
			return TunnelStatus{}, config.Remote{}, nil, config.AppConfig{}, err
		}
		key = &resolvedKey
	}
	return ts, remote, key, appCfg, nil
}

func sshArgs(tunnel config.Tunnel, remote config.Remote, key *config.SSHKey, appCfg config.AppConfig, reg *Registry) []string {
	forward := fmt.Sprintf("%s:%d:%s:%d", tunnel.BindAddress, tunnel.BindPort, tunnel.TargetHost, tunnel.TargetPort)

	args := []string{
		"-N",
		"-o", "BatchMode=yes",
		"-o", "PasswordAuthentication=no",
		"-o", "KbdInteractiveAuthentication=no",
		"-o", "NumberOfPasswordPrompts=0",
		"-o", "ExitOnForwardFailure=yes",
		// Keepalives let ssh notice a half-dead connection and exit, so a stale
		// forward (especially -R, whose remote listener otherwise lingers) gets
		// torn down and — for auto_start tunnels — re-established automatically.
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=3",
		"-o", "TCPKeepAlive=yes",
	}
	args = append(args, sshHostKeyArgs(appCfg)...)
	if key != nil {
		args = append(args,
			"-i", reg.KeyPath(key.File),
			"-o", "IdentitiesOnly=yes",
		)
	}
	args = append(args, string(tunnel.Direction), forward)
	args = append(args, tunnel.SSHOptions...)
	args = append(args, "-p", fmt.Sprintf("%d", remote.Port), fmt.Sprintf("%s@%s", remote.User, remote.Host))
	return args
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

func ensurePrivateKeyFile(path string, mode os.FileMode) error {
	if path == "" {
		return fmt.Errorf("ssh key file path is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("ssh key file %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("ssh key file %s is a directory", path)
	}
	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf("chmod ssh key file %s: %w", path, err)
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
		if strings.Contains(lower, "password") || strings.Contains(lower, "keyboard-interactive") {
			return "ssh authentication failed; the remote requires password/keyboard-interactive auth, which the service cannot use because it runs ssh non-interactively — configure key-based authentication for this remote"
		}
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

var shellSafeArgPattern = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

func shellCommand(bin string, args []string) string {
	quoted := make([]string, 0, len(args)+1)
	quoted = append(quoted, shellQuote(bin))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(arg string) string {
	if arg == "" {
		return "''"
	}
	if shellSafeArgPattern.MatchString(arg) {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
}
