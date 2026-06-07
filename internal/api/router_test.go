package api

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"testing"
)

func TestBootstrapUsesRemotePeerNotForwardedHeaders(t *testing.T) {
	router := NewRouter(Options{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		APIToken: "secret",
	})

	req, err := http.NewRequest(http.MethodGet, "/api/bootstrap", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.RemoteAddr = "203.0.113.10:54321"
	req.Header.Set("X-Forwarded-For", "127.0.0.1")
	rec := newResponseRecorder()

	router.ServeHTTP(rec, req)

	if rec.code != http.StatusForbidden {
		t.Fatalf("bootstrap with spoofed forwarded header returned %d, want %d", rec.code, http.StatusForbidden)
	}
}

func TestBootstrapAllowsLoopbackPeers(t *testing.T) {
	router := NewRouter(Options{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		APIToken: "secret",
	})

	for _, remoteAddr := range []string{"127.0.0.1:12345", "[::1]:12345"} {
		req, err := http.NewRequest(http.MethodGet, "/api/bootstrap", nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		req.RemoteAddr = remoteAddr
		rec := newResponseRecorder()

		router.ServeHTTP(rec, req)

		if rec.code != http.StatusOK {
			t.Fatalf("bootstrap from %s returned %d, want %d", remoteAddr, rec.code, http.StatusOK)
		}
	}
}

type responseRecorder struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{header: http.Header{}, code: http.StatusOK}
}

func (r *responseRecorder) Header() http.Header { return r.header }

func (r *responseRecorder) Write(p []byte) (int, error) {
	return r.body.Write(p)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.code = statusCode
}
