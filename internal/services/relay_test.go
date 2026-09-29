package services

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"ssh-tunnel-service/internal/config"
	"ssh-tunnel-service/internal/paths"
)

var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// echoServer accepts connections on l and echoes every byte back.
func echoServer(t *testing.T, l net.Listener) {
	t.Helper()
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}()
		}
	}()
}

func freePort(t *testing.T) int {
	t.Helper()
	p, err := freeLoopbackPort()
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	return p
}

// roundTrip sends payload over addr, half-closes, and returns what came back.
func roundTrip(t *testing.T, addr string, payload []byte) []byte {
	t.Helper()
	c, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = c.(*net.TCPConn).CloseWrite()
	got, err := io.ReadAll(c)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return got
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestRelayRemoteForwardMetersTraffic(t *testing.T) {
	target, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	echoServer(t, target)
	targetPort := target.Addr().(*net.TCPAddr).Port

	meter := &Meter{}
	r, err := newRelay(config.Tunnel{
		Direction: config.DirectionRemote, BindAddress: "0.0.0.0", BindPort: 9000,
		TargetHost: "127.0.0.1", TargetPort: targetPort,
	}, meter, discardLogger)
	if err != nil {
		t.Fatalf("newRelay: %v", err)
	}
	defer r.Close()

	// ssh would forward the remote port to the relay's loopback listener.
	relayAddr := r.listeners[0].Addr().String()
	if want := "0.0.0.0:9000:127.0.0.1:" + strconv.Itoa(r.listeners[0].Addr().(*net.TCPAddr).Port); r.forward != want {
		t.Fatalf("forward = %q, want %q", r.forward, want)
	}

	payload := bytes.Repeat([]byte("x"), 100_000)
	if got := roundTrip(t, relayAddr, payload); !bytes.Equal(got, payload) {
		t.Fatalf("echo mismatch: got %d bytes", len(got))
	}
	waitFor(t, "connection to close", func() bool { return meter.active.Load() == 0 })
	if up, down := meter.up.Load(), meter.down.Load(); up != 100_000 || down != 100_000 {
		t.Fatalf("up=%d down=%d, want 100000 each", up, down)
	}
	if meter.total.Load() != 1 {
		t.Fatalf("total conns = %d, want 1", meter.total.Load())
	}
}

func TestRelayLocalForwardWaitsForSSHAndReleasesPort(t *testing.T) {
	bindPort := freePort(t)
	meter := &Meter{}
	r, err := newRelay(config.Tunnel{
		Direction: config.DirectionLocal, BindAddress: "127.0.0.1", BindPort: bindPort,
		TargetHost: "db.internal", TargetPort: 5432,
	}, meter, discardLogger)
	if err != nil {
		t.Fatalf("newRelay: %v", err)
	}

	// ssh is told to listen on an internal loopback port and forward to the
	// real target.
	parts := strings.Split(r.forward, ":")
	if len(parts) != 4 || parts[0] != "127.0.0.1" || parts[2] != "db.internal" || parts[3] != "5432" {
		t.Fatalf("unexpected forward spec %q", r.forward)
	}

	// A client arriving before ssh has opened its listener waits for it.
	go func() {
		time.Sleep(300 * time.Millisecond)
		l, err := net.Listen("tcp", "127.0.0.1:"+parts[1])
		if err != nil {
			t.Errorf("fake ssh listen: %v", err)
			return
		}
		echoServer(t, l)
	}()
	if got := roundTrip(t, "127.0.0.1:"+strconv.Itoa(bindPort), []byte("hello")); string(got) != "hello" {
		t.Fatalf("echo = %q", got)
	}
	waitFor(t, "connection to close", func() bool { return meter.active.Load() == 0 })
	if meter.up.Load() != 5 || meter.down.Load() != 5 {
		t.Fatalf("up=%d down=%d, want 5 each", meter.up.Load(), meter.down.Load())
	}

	r.Close()
	l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(bindPort))
	if err != nil {
		t.Fatalf("bind port not released after Close: %v", err)
	}
	_ = l.Close()
}

func TestRelayLocalForwardReportsBusyPort(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	_, err = newRelay(config.Tunnel{
		Direction: config.DirectionLocal, BindAddress: "127.0.0.1", BindPort: busy.Addr().(*net.TCPAddr).Port,
		TargetHost: "127.0.0.1", TargetPort: 1,
	}, &Meter{}, discardLogger)
	if err == nil || !strings.Contains(err.Error(), "cannot listen on") {
		t.Fatalf("expected a bind error, got %v", err)
	}
}

func TestSampleTrafficRatesAndTotals(t *testing.T) {
	rt := NewRuntime()
	a := rt.Meter("a")
	b := rt.Meter("b")
	t0 := time.Unix(1_700_000_000, 0)

	rt.sampleTraffic(t0)
	a.up.Add(2000)
	a.down.Add(500)
	b.down.Add(1000)
	b.active.Add(1)
	rt.sampleTraffic(t0.Add(2 * time.Second))

	ca := rt.TunnelTraffic("a")
	if ca == nil || ca.UpRate != 1000 || ca.DownRate != 250 {
		t.Fatalf("tunnel a counters = %+v", ca)
	}
	snap := rt.TrafficSnapshot(true)
	if snap.Total.DownRate != 750 || snap.Total.UpBytes != 2000 || snap.Total.ActiveConns != 1 {
		t.Fatalf("total = %+v", snap.Total)
	}
	if len(snap.History.Total) != 2 || len(snap.History.Tunnels["b"]) != 2 {
		t.Fatalf("history = %+v", snap.History)
	}

	// A rename keeps the counters; a delete keeps the aggregate monotonic.
	rt.RenameTraffic("a", "a2")
	if rt.TunnelTraffic("a") != nil || rt.TunnelTraffic("a2").UpBytes != 2000 {
		t.Fatalf("rename did not move the meter")
	}
	rt.DropTraffic("a2")
	if got := rt.TrafficSnapshot(false).Total.UpBytes; got != 2000 {
		t.Fatalf("total up bytes after drop = %d, want 2000", got)
	}
	if rt.TunnelTraffic("unknown") != nil {
		t.Fatalf("unmetered tunnel should report nil traffic")
	}
}

func TestSubscribeTrafficReceivesSamples(t *testing.T) {
	rt := NewRuntime()
	rt.Meter("a").up.Add(10)
	ch, cancel := rt.SubscribeTraffic()
	defer cancel()
	rt.sampleTraffic(time.Now())
	select {
	case snap := <-ch:
		if snap.Tunnels["a"].UpBytes != 10 || snap.History != nil {
			t.Fatalf("unexpected snapshot %+v", snap)
		}
	case <-time.After(time.Second):
		t.Fatal("no snapshot delivered")
	}
}

// TestManagerRoutesLocalForwardThroughRelay checks the ssh process is pointed
// at the relay's internal port while the service owns the user-facing port,
// and that a direct tunnel still hands ssh its own spec.
func TestManagerRoutesLocalForwardThroughRelay(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake ssh shell script is POSIX-only")
	}
	home := t.TempDir()
	argsFile := filepath.Join(home, "args")
	binDir := t.TempDir()
	script := "#!/bin/sh\n" +
		"echo \"$@\" > \"" + argsFile + "\"\n" +
		"trap 'exit 0' TERM\n" +
		"while true; do sleep 0.05; done\n"
	if err := os.WriteFile(filepath.Join(binDir, "ssh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	bindPort := freePort(t)
	directPort := freePort(t)
	cfg := &config.Config{
		App:     config.AppConfig{SSHHostKeyPolicy: config.SSHHostKeyPolicyInsecure},
		Remotes: []config.Remote{{Name: "remote-a", Host: "ssh.example.com", Port: 22, User: "ubuntu"}},
		Tunnels: []config.Tunnel{
			{Name: "relayed", Remote: "remote-a", Direction: config.DirectionLocal, BindAddress: "127.0.0.1", BindPort: bindPort, TargetHost: "db.internal", TargetPort: 5432},
			{Name: "direct", Remote: "remote-a", Direction: config.DirectionLocal, BindAddress: "127.0.0.1", BindPort: directPort, TargetHost: "db.internal", TargetPort: 5432, Direct: true},
		},
	}
	rt := NewRuntime()
	reg := New(cfg, paths.Paths{Home: home}, rt)
	mgr := NewManager(context.Background(), reg, rt, discardLogger, false)
	reg.SetManager(mgr)

	readArgs := func() string {
		var data []byte
		waitFor(t, "fake ssh to record its args", func() bool {
			var err error
			data, err = os.ReadFile(argsFile)
			return err == nil && len(data) > 0
		})
		_ = os.Remove(argsFile)
		return string(data)
	}

	if err := mgr.Start("relayed"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	got := readArgs()
	if strings.Contains(got, "127.0.0.1:"+strconv.Itoa(bindPort)+":") || !strings.Contains(got, ":db.internal:5432") {
		t.Fatalf("ssh should get the relay's internal forward, got %q", got)
	}
	// The service now holds the user-facing port.
	if l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(bindPort)); err == nil {
		_ = l.Close()
		t.Fatalf("expected the relay to own port %d while running", bindPort)
	}
	if ts, _ := reg.GetTunnel("relayed"); ts.Traffic == nil {
		t.Fatalf("relayed tunnel should report traffic")
	}
	// The command preview still shows the tunnel's own spec.
	preview, err := mgr.Command("relayed")
	if err != nil || !strings.Contains(preview.Command, "-L 127.0.0.1:"+strconv.Itoa(bindPort)+":db.internal:5432") {
		t.Fatalf("preview = %q (%v)", preview.Command, err)
	}
	if err := mgr.Stop("relayed"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(bindPort))
	if err != nil {
		t.Fatalf("port not released after Stop: %v", err)
	}
	_ = l.Close()

	if err := mgr.Start("direct"); err != nil {
		t.Fatalf("Start direct: %v", err)
	}
	if got := readArgs(); !strings.Contains(got, "-L 127.0.0.1:"+strconv.Itoa(directPort)+":db.internal:5432") {
		t.Fatalf("direct tunnel should get its own spec, got %q", got)
	}
	if ts, _ := reg.GetTunnel("direct"); ts.Traffic != nil {
		t.Fatalf("direct tunnel should not report traffic")
	}
	mgr.Shutdown()
}

func TestFreeLocalPortSkipsClaimedAndBusyPorts(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	busyPort := busy.Addr().(*net.TCPAddr).Port
	cfg := &config.Config{
		Remotes: []config.Remote{{Name: "r", Host: "h", Port: 22, User: "u"}},
		Tunnels: []config.Tunnel{{Name: "t", Remote: "r", Direction: config.DirectionLocal, BindAddress: "127.0.0.1", BindPort: busyPort + 1, TargetHost: "x", TargetPort: 1}},
	}
	reg := New(cfg, paths.Paths{Home: t.TempDir()}, NewRuntime())
	got, err := reg.FreeLocalPort(busyPort)
	if err != nil {
		t.Fatalf("FreeLocalPort: %v", err)
	}
	if got == busyPort || got == busyPort+1 {
		t.Fatalf("FreeLocalPort returned a busy or claimed port %d", got)
	}
}
