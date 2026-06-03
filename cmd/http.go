package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
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
}

// newAPIClient locates a running service to talk to.
//
//   - An explicit --home / SSH_TUNNEL_HOME selects that specific instance: the
//     address comes from its config.yaml and the token from its token file
//     (falling back to loopback bootstrap if the file is unreadable, e.g. a
//     non-root user addressing the system instance by path).
//   - Otherwise we use runtime discovery: the user instance is tried before the
//     system one (endpoint.Discover order), obtaining the token via the
//     loopback-only /api/bootstrap so no access to the service's home is needed.
//   - As a last resort we fall back to the default home, preserving the original
//     same-user behavior and "no service running" hint.
func newAPIClient(home string) (*apiClient, error) {
	if home != "" || os.Getenv("SSH_TUNNEL_HOME") != "" {
		return apiClientFromHome(home)
	}
	for _, info := range endpoint.Discover() {
		c := &apiClient{base: "http://" + info.Address, listen: info.Address}
		if err := c.resolveToken(); err == nil {
			return c, nil
		}
	}
	return apiClientFromHome("")
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
