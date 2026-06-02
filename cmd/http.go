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

// newAPIClient resolves the service's listen address and API token from the home
// directory. It never creates or rewrites config — a missing config simply means
// no service has run yet, and the request will surface the "not running" hint.
func newAPIClient(home string) (*apiClient, error) {
	p, err := paths.Resolve(home)
	if err != nil {
		return nil, err
	}
	listen := config.DefaultHTTPListen
	if cfg, err := config.LoadWithDefaults(p.Config(), p.KnownHosts()); err == nil {
		listen = cfg.App.HTTPListen
	}
	token := ""
	if raw, err := os.ReadFile(p.Token()); err == nil {
		token = strings.TrimSpace(string(raw))
	}
	return &apiClient{base: "http://" + listen, listen: listen, token: token}, nil
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
