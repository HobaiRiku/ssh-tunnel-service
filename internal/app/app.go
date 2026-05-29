// Package app is the composition root: paths -> config -> log -> services -> HTTP.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/HobaiRiku/ssh-tunnel-service/internal/api"
	"github.com/HobaiRiku/ssh-tunnel-service/internal/config"
	"github.com/HobaiRiku/ssh-tunnel-service/internal/paths"
	"github.com/HobaiRiku/ssh-tunnel-service/internal/services"
	"github.com/HobaiRiku/ssh-tunnel-service/internal/version"
	"github.com/HobaiRiku/ssh-tunnel-service/internal/web"
)

// Options bundles the inputs Run needs. Built by cmd/ from CLI flags.
type Options struct {
	Paths    paths.Paths
	Config   *config.Config
	Logger   *slog.Logger
	APIToken string // loaded from the token file, not from config.yaml
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
	mgr := services.NewManager(reg, rt, opts.Logger.With("component", "manager"))
	reg.SetManager(mgr)

	// AutoStart tunnels marked with auto_start: true
	mgr.AutoStart(ctx)

	router := api.NewRouter(api.Options{
		Context:  ctx,
		Registry: reg,
		Runtime:  rt,
		Manager:  mgr,
		Logger:   opts.Logger.With("component", "api"),
		APIToken: opts.APIToken,
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

	opts.Logger.Info("management api listening", "addr", opts.Config.App.HTTPListen)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}
