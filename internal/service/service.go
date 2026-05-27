package service

import (
"context"
"errors"
"fmt"
"io"
"log/slog"
"runtime"
"sync"

kservice "github.com/kardianos/service"

"github.com/HobaiRiku/ssh-tunnel-service/internal/app"
"github.com/HobaiRiku/ssh-tunnel-service/internal/config"
applog "github.com/HobaiRiku/ssh-tunnel-service/internal/log"
"github.com/HobaiRiku/ssh-tunnel-service/internal/paths"
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
}
if runtime.GOOS == "darwin" {
cfg.Option["UserService"] = true
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

// Install registers the service with the OS service manager.
func Install(home string) error {
svc, _, err := New(home)
if err != nil {
return err
}
darwinBootout()
if err := svc.Install(); err != nil {
return err
}
return darwinBootstrap()
}

// Uninstall removes the service registration.
func Uninstall(home string) error {
svc, _, err := New(home)
if err != nil {
return err
}
darwinBootout()
return svc.Uninstall()
}

// Start requests the OS to start the registered service.
func Start(home string) error {
if started, err := darwinKickstart(); started {
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
if stopped, err := darwinKill(); stopped {
return err
}
svc, _, err := New(home)
if err != nil {
return err
}
return svc.Stop()
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

cfg, err := config.Load(p.Config())
if err != nil {
var miss *config.MissingFileError
if errors.As(err, &miss) {
if err := config.WriteExample(p.Config(), p.FileMode()); err != nil {
return app.Options{}, nil, fmt.Errorf("init config at %s: %w", miss.Path, err)
}
cfg, err = config.Load(p.Config())
if err != nil {
return app.Options{}, nil, fmt.Errorf("load initialized config: %w", err)
}
} else {
return app.Options{}, nil, err
}
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

return app.Options{Paths: p, Config: cfg, Logger: logger}, closer, nil
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
