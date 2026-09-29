package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"

	"ssh-tunnel-service/internal/config"
	"ssh-tunnel-service/internal/paths"
	"ssh-tunnel-service/internal/services"
)

func TestTrafficStreamAcceptsBearerSubprotocol(t *testing.T) {
	rt := services.NewRuntime()
	rt.Meter("db")
	srv := httptest.NewServer(NewRouter(Options{
		Context:  context.Background(),
		Runtime:  rt,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		APIToken: "secret",
	}))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/traffic/stream"

	// Without credentials the handshake is refused.
	if _, resp, err := websocket.DefaultDialer.Dial(url, nil); err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a token, got err=%v resp=%v", err, resp)
	}

	// A browser authenticates by offering the token as a subprotocol.
	dialer := websocket.Dialer{Subprotocols: []string{"ssh-tunnel", "bearer.secret"}}
	conn, resp, err := dialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial with bearer subprotocol: %v", err)
	}
	defer conn.Close()
	if got := resp.Header.Get("Sec-WebSocket-Protocol"); got != "ssh-tunnel" {
		t.Fatalf("selected subprotocol = %q, want ssh-tunnel", got)
	}
	var snap services.TrafficSnapshot
	if err := conn.ReadJSON(&snap); err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if _, ok := snap.Tunnels["db"]; !ok || snap.History == nil {
		t.Fatalf("initial snapshot should list tunnel db with history, got %+v", snap)
	}
}

func TestTrafficHistoryValidatesRangeAndTunnel(t *testing.T) {
	rt := services.NewRuntime()
	router := NewRouter(Options{
		Context:  context.Background(),
		Runtime:  rt,
		Registry: services.New(&config.Config{}, paths.Paths{Home: t.TempDir()}, rt),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	for path, want := range map[string]int{
		"/api/traffic/history?range=24h":   http.StatusOK,
		"/api/traffic/history?range=7d":    http.StatusBadRequest,
		"/api/traffic/history?tunnel=nope": http.StatusNotFound,
		"/api/traffic/usage":               http.StatusOK,
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != want {
			t.Errorf("GET %s = %d, want %d (%s)", path, rec.Code, want, rec.Body.String())
		}
	}
}
