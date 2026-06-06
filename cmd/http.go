package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"ssh-tunnel-service/internal/config"
	"ssh-tunnel-service/internal/endpoint"
	"ssh-tunnel-service/internal/paths"
)

// apiClient talks to the running service's REST API. Every resource operation
// the CLI performs (keys, remotes, tunnels — list as well as mutations) goes
// through the live backend rather than editing config.yaml directly. Editing the
// file behind a running service would drift from its in-memory state and be
// silently overwritten on the service's next write, so all of these commands
// require the service to be running.
type apiClient struct {
	base   string
	listen string
	token  string
	scope  string // "system" | "user" | "custom" (for the instance banner)
	home   string // resolved data root, when known
}

// bannerSuppressed silences the per-command instance banner. Commands that
// build their own client for inspection (e.g. `connect --show`) set it.
var bannerSuppressed bool

// newAPIClient resolves the target instance and prints the instance banner to
// stderr (unless suppressed). See resolveClient for the attach precedence.
func newAPIClient(home string) (*apiClient, error) {
	c, err := resolveClient(home)
	if err != nil {
		return nil, err
	}
	if !bannerSuppressed {
		fmt.Fprintln(os.Stderr, c.banner())
	}
	return c, nil
}

// resolveClient locates a running service to talk to, in precedence order:
//
//		--home flag  →  SSH_TUNNEL_HOME env  →  persisted context  →  auto-discovery
//
//	  - An explicit --home / SSH_TUNNEL_HOME is a one-shot override selecting that
//	    specific instance by path (token from its token file, falling back to
//	    loopback bootstrap if unreadable). It does not mutate the context.
//	  - A persisted context (set via `connect`) pins system/user/custom. An
//	    unhealthy pinned instance does not hard-fail: we print a notice and fall
//	    through to auto-discovery.
//	  - Auto-discovery tries the user instance before the system one, obtaining
//	    the token via the loopback-only /api/bootstrap.
//	  - As a last resort we fall back to the default home, preserving the "no
//	    service running" hint.
func resolveClient(home string) (*apiClient, error) {
	if home != "" {
		return clientForHome(home, scopeCustom)
	}
	if env := os.Getenv("SSH_TUNNEL_HOME"); env != "" {
		return clientForHome(env, scopeCustom)
	}

	switch ctx := loadContext(); ctx.Scope {
	case scopeSystem, scopeUser:
		if epScope, ok := scopeToEndpoint(ctx.Scope); ok {
			if c, err := clientForScope(epScope); err == nil {
				return c, nil
			}
		}
		fallbackNotice(ctx.Scope)
	case scopeCustom:
		if c, err := clientForHome(ctx.Home, scopeCustom); err == nil && c.healthy() {
			return c, nil
		}
		fallbackNotice(scopeCustom + " " + ctx.Home)
	}

	for _, info := range endpoint.Discover() {
		c := clientFromInfo(info)
		if err := c.resolveToken(); err == nil {
			return c, nil
		}
	}
	return apiClientFromHome("")
}

// fallbackNotice warns that a pinned context is unreachable and discovery is
// taking over, so the user understands why the active instance changed.
func fallbackNotice(what string) {
	fmt.Fprintf(os.Stderr,
		"ssh-tunnel: last connected instance (%s) is not reachable; falling back to auto-discovery\n", what)
}

// clientFromInfo builds a client from a discovery record.
func clientFromInfo(info endpoint.Info) *apiClient {
	return &apiClient{
		base:   "http://" + info.Address,
		listen: info.Address,
		scope:  string(info.Scope),
		home:   info.Home,
	}
}

// clientForScope resolves a scope's advertised endpoint and authenticates.
func clientForScope(scope endpoint.Scope) (*apiClient, error) {
	info, ok := endpoint.Lookup(scope)
	if !ok {
		return nil, fmt.Errorf("no %s instance is running", scope)
	}
	c := clientFromInfo(info)
	if err := c.resolveToken(); err != nil {
		return nil, err
	}
	return c, nil
}

// clientForHome builds a client targeting a specific data root, labelling it
// with the given scope for the banner.
func clientForHome(home, scope string) (*apiClient, error) {
	c, err := apiClientFromHome(home)
	if err != nil {
		return nil, err
	}
	c.scope = scope
	if p, err := paths.Resolve(home); err == nil {
		c.home = p.Home
	}
	return c, nil
}

// healthy reports whether the instance answers the unauthenticated health probe.
func (a *apiClient) healthy() bool {
	return a.request(http.MethodGet, "/api/health", nil, nil) == nil
}

// banner renders the one-line instance identity for stderr.
func (a *apiClient) banner() string {
	scope := a.scope
	if scope == "" {
		scope = "instance"
	}
	if scope == scopeCustom && a.home != "" {
		return fmt.Sprintf("[ssh-tunnel @ %s %s · %s]", scope, filepath.Base(a.home), a.listen)
	}
	return fmt.Sprintf("[ssh-tunnel @ %s · %s]", scope, a.listen)
}

// apiClientFromHome builds a client from a resolved home directory. It never
// creates or rewrites config — a missing config simply means no service has run
// yet, and the request will surface the "not running" hint.
func apiClientFromHome(home string) (*apiClient, error) {
	p, err := paths.Resolve(home)
	if err != nil {
		return nil, err
	}
	listen := config.DefaultHTTPListen
	if cfg, err := config.LoadWithDefaults(p.Config(), p.KnownHosts()); err == nil {
		listen = cfg.App.HTTPListen
	}
	listen = normalizeListen(listen)
	c := &apiClient{base: "http://" + listen, listen: listen}
	if raw, err := os.ReadFile(p.Token()); err == nil {
		c.token = strings.TrimSpace(string(raw))
	}
	if c.token == "" {
		// Token file unreadable (e.g. addressing the root-owned system instance
		// as a normal user); try the loopback bootstrap instead.
		_ = c.resolveToken()
	}
	return c, nil
}

// resolveToken fetches the API token from the loopback-only bootstrap endpoint.
// Success doubles as a liveness check, so discovery can use it to pick a
// reachable instance.
func (a *apiClient) resolveToken() error {
	var out struct {
		Token string `json:"token"`
	}
	if err := a.request(http.MethodGet, "/api/bootstrap", nil, &out); err != nil {
		return err
	}
	if out.Token == "" {
		return errors.New("bootstrap returned an empty token")
	}
	a.token = out.Token
	return nil
}

// normalizeListen rewrites a wildcard bind address to loopback so the CLI can
// connect and satisfy the bootstrap origin check.
func normalizeListen(addr string) string {
	host, port, found := strings.Cut(addr, ":")
	if !found {
		return addr
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		return "127.0.0.1:" + port
	}
	return addr
}

// request performs an API call. On a 2xx response it decodes the JSON body into
// out (when non-nil). A transport error is reported as "service not running";
// a non-2xx response is turned into the server's error message.
func (a *apiClient) request(method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, a.base+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("no service is running at %s — start it with `ssh-tunnel start` (or run `ssh-tunnel run` in the foreground)", a.listen)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return responseError(resp)
	}
	if out != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func responseError(resp *http.Response) error {
	data, _ := io.ReadAll(resp.Body)
	var payload struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(data, &payload) == nil && payload.Error != "" {
		return errors.New(payload.Error)
	}
	return fmt.Errorf("server returned %s", resp.Status)
}
