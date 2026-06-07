// Package app is the composition root: paths -> config -> log -> services -> HTTP.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"ssh-tunnel-service/internal/api"
	"ssh-tunnel-service/internal/config"
	"ssh-tunnel-service/internal/endpoint"
	"ssh-tunnel-service/internal/paths"
	"ssh-tunnel-service/internal/services"
	"ssh-tunnel-service/internal/version"
	"ssh-tunnel-service/internal/web"
)

// Options bundles the inputs Run needs. Built by cmd/ from CLI flags.
type Options struct {
	Paths    paths.Paths
	Config   *config.Config
	Logger   *slog.Logger
	APIToken string // loaded from the token file, not from config.yaml
	// SystemService is true when running as an elevated system service, which
	// enables the system default key for unbound tunnels (see Manager).
	SystemService bool
}

// Run starts the tunnel manager and HTTP server and blocks until ctx is cancelled.
func Run(ctx context.Context, opts Options) error {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	info := version.Current()
	opts.Logger.Info("ssh-tunnel-service starting", "version", info.Version, "home", opts.Paths.Home)
	opts.Logger.Info("api token loaded", "token_file", opts.Paths.Token())

	rt := services.NewRuntime()
	reg := services.New(opts.Config, opts.Paths, rt)
	mgr := services.NewManager(ctx, reg, rt, opts.Logger.With("component", "manager"), opts.SystemService)
	reg.SetManager(mgr)

	// Backfill `.pub` files for any managed key created before public-key
	// materialization existed, so the web UI / `key pub` can surface them.
	reg.EnsurePublicKeys()

	// As a system service, guarantee the managed default key exists before any
	// unbound tunnel tries to use it.
	if opts.SystemService {
		if name, err := reg.EnsureSystemDefaultKey(); err != nil {
			return fmt.Errorf("ensure system default key: %w", err)
		} else {
			opts.Logger.Info("system default key ready", "key", name)
		}
	}

	// AutoStart tunnels marked with auto_start: true; the manager then keeps
	// them alive (reconnecting on unexpected drops) until ctx is cancelled.
	mgr.AutoStart()

	router := api.NewRouter(api.Options{
		Context:  ctx,
		Registry: reg,
		Runtime:  rt,
		Manager:  mgr,
		Logger:   opts.Logger.With("component", "api"),
		APIToken: opts.APIToken,
		Scope:    string(endpoint.CurrentScope()),
		Home:     opts.Paths.Home,
		Address:  opts.Config.App.HTTPListen,
		Started:  time.Now(),
		LogFile:  opts.Paths.LogFile(),
	})
	web.Mount(router, opts.APIToken)

	srv := &http.Server{
		Addr:              opts.Config.App.HTTPListen,
		Handler:           router,
		ReadHeaderTimeout: 15 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	// Advertise this instance for CLI discovery (home-independent). Best-effort:
	// a failure only means clients must fall back to --home/SSH_TUNNEL_HOME.
	if path, err := endpoint.Write(opts.Config.App.HTTPListen, opts.Paths.Home); err != nil {
		opts.Logger.Warn("could not advertise endpoint for CLI discovery", "err", err)
	} else {
		opts.Logger.Info("endpoint advertised", "path", path, "scope", endpoint.CurrentScope())
		defer endpoint.Remove()
	}

	opts.Logger.Info("management api listening", "addr", opts.Config.App.HTTPListen)
	err := srv.ListenAndServe()
	// The HTTP server has stopped (ctx cancelled or a fatal listen error). Tear
	// down every ssh child before returning so a subsequent service start is not
	// blocked by an orphaned process still holding a forwarded port.
	mgr.Shutdown()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}
