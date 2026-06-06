package api

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// logUpgrader upgrades the log-stream request to a WebSocket. The handshake has
// already passed tokenAuth (the Authorization header is checked before the
// upgrade), so origin checks add nothing here for a loopback management API.
var logUpgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

// streamLogs serves GET /api/logs/stream: it sends the last `lines` lines of the
// service log, then (unless follow=false) streams appended content as it is
// written, so the CLI `tail` never needs filesystem access to the data root.
func streamLogs(logFile string) gin.HandlerFunc {
	return func(c *gin.Context) {
		lines := 50
		if v := c.Query("lines"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				lines = n
			}
		}
		follow := c.Query("follow") != "false"

		conn, err := logUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()

		// A reader goroutine surfaces client disconnects (and consumes any control
		// frames) so the streaming loop below can stop promptly.
		go func() {
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					cancel()
					return
				}
			}
		}()

		send := func(p []byte) error {
			return conn.WriteMessage(websocket.TextMessage, p)
		}
		_ = streamTail(ctx, logFile, lines, follow, send)
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(time.Second))
	}
}

// streamTail emits the last `lines` lines of path, then follows appended bytes
// until ctx is cancelled. send delivers each chunk; a send error stops the loop.
func streamTail(ctx context.Context, path string, lines int, follow bool, send func([]byte) error) error {
	if initial, err := lastLines(path, lines); err == nil && len(initial) > 0 {
		if err := send(initial); err != nil {
			return err
		}
	}
	if !follow {
		return nil
	}

	var offset int64
	if info, err := os.Stat(path); err == nil {
		offset = info.Size()
	}
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()
	buf := make([]byte, 32*1024)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		f, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				offset = 0
				continue
			}
			return err
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return err
		}
		if info.Size() < offset {
			offset = 0 // file was rotated/truncated
		}
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			f.Close()
			return err
		}
		for {
			n, rerr := f.Read(buf)
			if n > 0 {
				if err := send(buf[:n]); err != nil {
					f.Close()
					return err
				}
				offset += int64(n)
			}
			if rerr != nil {
				break
			}
		}
		f.Close()
	}
}

// lastLines returns the final n lines of path as a single newline-terminated
// blob, or nil when n is 0 or the file is empty.
func lastLines(path string, n int) ([]byte, error) {
	if n == 0 {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	ring := make([]string, 0, n)
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for s.Scan() {
		ring = append(ring, s.Text())
		if len(ring) > n {
			ring = ring[1:]
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	if len(ring) == 0 {
		return nil, nil
	}
	return []byte(strings.Join(ring, "\n") + "\n"), nil
}
