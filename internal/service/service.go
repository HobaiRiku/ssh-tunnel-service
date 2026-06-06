package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	kservice "github.com/kardianos/service"

	"ssh-tunnel-service/internal/app"
	"ssh-tunnel-service/internal/config"
	"ssh-tunnel-service/internal/elevate"
	applog "ssh-tunnel-service/internal/log"
	"ssh-tunnel-service/internal/paths"
)

const (
	serviceName        = "ssh-tunnel-service"
	serviceDisplayName = "SSH Tunnel Service"
	serviceDescription = "SSH port-forwarding tunnel manager"
)

// Program implements kardianos/service.Interface.
type Program struct {
	home string
	run  func(context.Context, string, bool) error

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
	runErr error
}

func NewProgram(home string) *Program {
	return &Program{home: home, run: Run}
}

func New(home string) (kservice.Service, *Program, error) {
	return newWithExe(home, "")
}

// newWithExe builds the kardianos service, optionally pinning the unit to a
// specific executable path (used after install copies the binary to a stable
// system location). An empty exe means "use the current executable".
//
// The service is registered as a system-level service on every platform
// (systemd system unit, macOS LaunchDaemon, Windows SCM service) so it survives
// reboots without an interactive login and is independent of the installing
// user. Privilege elevation for the control commands is handled in cmd/.
func newWithExe(home, exe string) (kservice.Service, *Program, error) {
	p, resolved, err := newProgram(home)
	if err != nil {
		return nil, nil, err
	}
	cfg := &kservice.Config{
		Name:             serviceName,
		DisplayName:      serviceDisplayName,
		Description:      serviceDescription,
		WorkingDirectory: resolved.Home,
		EnvVars:          map[string]string{"SSH_TUNNEL_HOME": resolved.Home},
		Option:           kservice.KeyValue{},
		Executable:       exe,
	}
	svc, err := kservice.New(p, cfg)
	if err != nil {
		return nil, nil, err
	}
	return svc, p, nil
}

func newProgram(home string) (*Program, paths.Paths, error) {
	resolved, err := paths.Resolve(home)
	if err != nil {
		return nil, paths.Paths{}, err
	}
	return NewProgram(resolved.Home), resolved, nil
}

// Interactive reports whether the current process is attached to a user session.
func Interactive() bool { return kservice.Interactive() }

// RunService enters kardianos/service managed mode.
func RunService(home string) error {
	svc, _, err := New(home)
	if err != nil {
		return err
	}
	return svc.Run()
}

// Install registers the system service. It first copies the running binary to a
// stable, system-wide location (see binary.go) so the generated unit references
// that path rather than the caller's working copy, then registers the service
// pointing at it. The binary copy is rolled back if registration fails.
func Install(home string) error {
	if err := validateInstallExecutable(); err != nil {
		return err
	}
	installedExe, created, err := installSystemBinary()
	if err != nil {
		return fmt.Errorf("install binary: %w", err)
	}
	svc, _, err := newWithExe(home, installedExe)
	if err != nil {
		rollbackBinary(installedExe, created)
		return err
	}
	darwinBootout()
	if err := svc.Install(); err != nil {
		rollbackBinary(installedExe, created)
		return err
	}
	if created {
		fmt.Fprintf(os.Stderr, "ssh-tunnel: binary installed to %s\n", installedExe)
	}
	if err := darwinBootstrap(); err != nil {
		// darwinBootstrap failed after the OS service was registered — undo
		// everything so the user can retry from a known-clean state.
		_ = svc.Uninstall()
		rollbackBinary(installedExe, created)
		return err
	}
	return nil
}

// Uninstall removes the service registration and the installed system binary.
func Uninstall(home string) error {
	svc, _, err := New(home)
	if err != nil {
		return err
	}
	darwinBootout()
	if err := svc.Uninstall(); err != nil {
		return err
	}
	return removeSystemBinary()
}

// rollbackBinary removes a freshly installed binary after a later install step
// failed, so a retry starts from a clean state. It only removes the binary when
// created is true — i.e. this invocation wrote it — to avoid deleting an
// existing binary that was already present before install ran.
func rollbackBinary(installedExe string, created bool) {
	if installedExe != "" && created {
		_ = removeSystemBinary()
	}
}

// Start requests the OS to start the registered service.
func Start(home string) error {
	if started, err := darwinStart(); started {
		return err
	}
	svc, _, err := New(home)
	if err != nil {
		return err
	}
	return svc.Start()
}

// Stop requests the OS to stop the registered service.
func Stop(home string) error {
	if stopped, err := darwinStop(); stopped {
		return err
	}
	svc, _, err := New(home)
	if err != nil {
		return err
	}
	return svc.Stop()
}

func validateInstallExecutable() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}
	resolved := exe
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		resolved = real
	}
	cleaned := filepath.Clean(resolved)
	tempRoots := []string{filepath.Clean(os.TempDir())}
	if realTemp, err := filepath.EvalSymlinks(os.TempDir()); err == nil {
		tempRoots = append(tempRoots, filepath.Clean(realTemp))
	}
	for _, root := range tempRoots {
		prefix := root
		if !strings.HasSuffix(prefix, string(os.PathSeparator)) {
			prefix += string(os.PathSeparator)
		}
		if strings.HasPrefix(cleaned, prefix) && strings.Contains(cleaned, string(os.PathSeparator)+"go-build") {
			return fmt.Errorf("refusing to install from temporary `go run` executable %s; build or install a stable binary first", resolved)
		}
	}
	return nil
}

// Status returns the OS service status.
func Status(home string) (kservice.Status, error) {
	svc, _, err := New(home)
	if err != nil {
		return kservice.StatusUnknown, err
	}
	return svc.Status()
}

// StatusString formats the kardianos status enum for CLI output.
func StatusString(status kservice.Status) string {
	switch status {
	case kservice.StatusRunning:
		return "running"
	case kservice.StatusStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// Run bootstraps paths/config/logging then blocks in app.Run.
func Run(ctx context.Context, home string, console bool) error {
	opts, closer, err := loadOptions(home, console)
	if err != nil {
		return err
	}
	defer closer.Close()

	opts.Logger.Info("ssh-tunnel-service starting", "home", opts.Paths.Home, "config", opts.Paths.Config())
	return app.Run(ctx, opts)
}

func loadOptions(home string, console bool) (app.Options, io.Closer, error) {
	p, err := paths.Resolve(home)
	if err != nil {
		return app.Options{}, nil, err
	}
	if err := p.EnsureTree(); err != nil {
		return app.Options{}, nil, fmt.Errorf("prepare home %s: %w", p.Home, err)
	}

	cfg, err := config.LoadWithDefaults(p.Config(), p.KnownHosts())
	if err != nil {
		var miss *config.MissingFileError
		if errors.As(err, &miss) {
			if err := config.WriteExample(p.Config(), p.FileMode()); err != nil {
				return app.Options{}, nil, fmt.Errorf("init config at %s: %w", miss.Path, err)
			}
			cfg, err = config.LoadWithDefaults(p.Config(), p.KnownHosts())
			if err != nil {
				return app.Options{}, nil, fmt.Errorf("load initialized config: %w", err)
			}
		} else {
			return app.Options{}, nil, err
		}
	}

	// Load or generate the API token from its own file so config.yaml is never rewritten.
	token, err := loadOrGenerateToken(p.Token(), p.FileMode())
	if err != nil {
		return app.Options{}, nil, fmt.Errorf("api token: %w", err)
	}

	logger, _, closer, err := applog.Init(applog.Options{
		Level:      cfg.App.LogLevel,
		File:       p.LogFile(),
		Console:    console || cfg.App.LogConsole,
		MaxSizeMB:  cfg.App.LogMaxSizeMB,
		MaxBackups: cfg.App.LogMaxBackups,
		MaxAgeDays: cfg.App.LogMaxAgeDays,
		Compress:   cfg.App.LogCompress,
	})
	if err != nil {
		return app.Options{}, nil, fmt.Errorf("log init: %w", err)
	}

	return app.Options{
		Paths:         p,
		Config:        cfg,
		Logger:        logger,
		APIToken:      token,
		SystemService: elevate.IsElevated(),
	}, closer, nil
}

// loadOrGenerateToken reads the token from tokenPath, creating it if absent.
func loadOrGenerateToken(tokenPath string, mode os.FileMode) (string, error) {
	data, err := os.ReadFile(tokenPath)
	if err == nil {
		if t := strings.TrimSpace(string(data)); t != "" {
			return t, nil
		}
	}
	t := config.GenerateToken()
	if err := os.WriteFile(tokenPath, []byte(t+"\n"), mode); err != nil {
		return "", fmt.Errorf("write token file %s: %w", tokenPath, err)
	}
	fmt.Fprintf(os.Stderr, "ssh-tunnel: generated new API token → %s\n", tokenPath)
	return t, nil
}

// Start implements kardianos/service.Interface.
func (p *Program) Start(_ kservice.Service) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		return errors.New("service already started")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	p.cancel = cancel
	p.done = done
	p.runErr = nil
	go func() {
		err := p.run(ctx, p.home, false)
		if err != nil && !errors.Is(err, context.Canceled) {
			slog.Default().Error("service exited", "err", err)
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		p.runErr = err
		p.cancel = nil
		close(done)
	}()
	return nil
}

// Stop implements kardianos/service.Interface.
func (p *Program) Stop(_ kservice.Service) error {
	p.mu.Lock()
	cancel := p.cancel
	done := p.done
	p.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	if done != nil {
		<-done
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.runErr
}
