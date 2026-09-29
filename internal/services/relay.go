package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"ssh-tunnel-service/internal/config"
)

// relayDialTimeout bounds how long a relayed connection waits for its upstream:
// the -R target service, or — for -L — ssh's internal listener, which only
// appears once ssh has authenticated and set up the forward.
const relayDialTimeout = 10 * time.Second

// relayBufSize is the per-direction copy buffer. Counting happens per read, so
// even a trickle of interactive traffic shows up in the next sample.
const relayBufSize = 32 * 1024

// relay routes a tunnel's connections through the service so their bytes can be
// metered. ssh still carries the traffic; the relay only sits on the plaintext
// loopback side of it:
//
//   - -L: the relay owns the user-facing bind_address:bind_port and ssh listens
//     on an internal loopback port instead; each accepted connection is spliced
//     to that port.
//   - -R: ssh forwards the remote port to a loopback port the relay owns, and
//     the relay dials the real target_host:target_port.
//
// The relay lives exactly as long as its ssh process: Manager.Start creates it
// and waitProcess closes it, so the user-facing port is released whenever the
// tunnel is not running — the same as when ssh held it directly.
type relay struct {
	// forward is the -L/-R spec handed to ssh in place of the tunnel's own.
	forward   string
	listeners []net.Listener
	dial      func(ctx context.Context) (net.Conn, error)
	meter     *Meter
	logger    *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	conns  map[net.Conn]struct{}
	closed bool
	wg     sync.WaitGroup
}

// newRelay builds and starts the relay for a tunnel. It fails when the
// user-facing port (-L) cannot be bound, which is reported before ssh is even
// spawned.
func newRelay(t config.Tunnel, meter *Meter, logger *slog.Logger) (*relay, error) {
	r := &relay{meter: meter, logger: logger, conns: map[net.Conn]struct{}{}}
	r.ctx, r.cancel = context.WithCancel(context.Background())

	switch t.Direction {
	case config.DirectionLocal:
		internal, err := freeLoopbackPort()
		if err != nil {
			return nil, fmt.Errorf("reserve internal port: %w", err)
		}
		listeners, err := listenBind(t.BindAddress, t.BindPort)
		if err != nil {
			return nil, err
		}
		r.listeners = listeners
		r.forward = fmt.Sprintf("127.0.0.1:%d:%s:%d", internal, t.TargetHost, t.TargetPort)
		upstream := net.JoinHostPort("127.0.0.1", strconv.Itoa(internal))
		r.dial = func(ctx context.Context) (net.Conn, error) { return dialRetry(ctx, upstream) }
	case config.DirectionRemote:
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, fmt.Errorf("open relay listener: %w", err)
		}
		r.listeners = []net.Listener{l}
		port := l.Addr().(*net.TCPAddr).Port
		r.forward = fmt.Sprintf("%s:%d:127.0.0.1:%d", t.BindAddress, t.BindPort, port)
		target := net.JoinHostPort(t.TargetHost, strconv.Itoa(t.TargetPort))
		r.dial = func(ctx context.Context) (net.Conn, error) {
			d := net.Dialer{Timeout: relayDialTimeout}
			return d.DialContext(ctx, "tcp", target)
		}
	default:
		return nil, fmt.Errorf("unsupported direction %q", t.Direction)
	}

	for _, l := range r.listeners {
		r.wg.Add(1)
		go r.serve(l)
	}
	return r, nil
}

// Close stops accepting, tears down every open connection and waits for the
// relay's goroutines to finish, so the bound ports are free when it returns.
func (r *relay) Close() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	r.cancel()
	for _, l := range r.listeners {
		_ = l.Close()
	}
	for c := range r.conns {
		_ = c.Close()
	}
	r.mu.Unlock()
	r.wg.Wait()
}

func (r *relay) serve(l net.Listener) {
	defer r.wg.Done()
	for {
		c, err := l.Accept()
		if err != nil {
			if r.ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			// Transient accept failures (e.g. EMFILE) must not kill the tunnel.
			r.logger.Warn("relay accept failed", "err", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if !r.track(c) {
			_ = c.Close()
			return
		}
		r.wg.Add(1)
		go r.handle(c)
	}
}

// track registers a connection for Close; it reports false once the relay is
// closing, in which case the caller must drop the connection itself.
func (r *relay) track(c net.Conn) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return false
	}
	r.conns[c] = struct{}{}
	return true
}

func (r *relay) untrack(c net.Conn) {
	r.mu.Lock()
	delete(r.conns, c)
	r.mu.Unlock()
	_ = c.Close()
}

func (r *relay) handle(client net.Conn) {
	defer r.wg.Done()
	defer r.untrack(client)
	r.meter.total.Add(1)
	r.meter.active.Add(1)
	defer r.meter.active.Add(-1)

	upstream, err := r.dial(r.ctx)
	if err != nil {
		if r.ctx.Err() == nil {
			r.logger.Warn("relay could not reach upstream", "err", err)
		}
		return
	}
	if !r.track(upstream) {
		_ = upstream.Close()
		return
	}
	defer r.untrack(upstream)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		pipe(upstream, client, &r.meter.up)
	}()
	go func() {
		defer wg.Done()
		pipe(client, upstream, &r.meter.down)
	}()
	wg.Wait()
}

// pipe copies src to dst, counting every chunk as it goes. A clean EOF is
// propagated as a half-close so request/response protocols finish normally;
// any other error aborts both sides so the opposite copy cannot hang.
func pipe(dst, src net.Conn, counter *atomic.Uint64) {
	buf := make([]byte, relayBufSize)
	for {
		n, rerr := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				_ = src.Close()
				_ = dst.Close()
				return
			}
			counter.Add(uint64(n))
		}
		if rerr == nil {
			continue
		}
		if errors.Is(rerr, io.EOF) {
			closeWrite(dst)
			return
		}
		_ = src.Close()
		_ = dst.Close()
		return
	}
}

func closeWrite(c net.Conn) {
	if cw, ok := c.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
		return
	}
	_ = c.Close()
}

// dialRetry connects to ssh's internal -L listener. ssh opens it only after it
// has authenticated, so a client that arrives in that window waits for it
// instead of being refused.
func dialRetry(ctx context.Context, addr string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, relayDialTimeout)
	defer cancel()
	var d net.Dialer
	for {
		c, err := d.DialContext(ctx, "tcp", addr)
		if err == nil {
			return c, nil
		}
		select {
		case <-ctx.Done():
			return nil, err
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// listenBind binds the user-facing side of a -L tunnel with the same address
// semantics ssh applies to a -L bind_address: "*" or empty means every
// interface, "localhost" means loopback on both IPv4 and IPv6.
func listenBind(bindAddress string, port int) ([]net.Listener, error) {
	hosts := []string{bindAddress}
	switch bindAddress {
	case "", "*":
		hosts = []string{""}
	case "localhost":
		hosts = []string{"127.0.0.1", "::1"}
	}
	var (
		listeners []net.Listener
		firstErr  error
	)
	for _, h := range hosts {
		l, err := net.Listen("tcp", net.JoinHostPort(h, strconv.Itoa(port)))
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		listeners = append(listeners, l)
	}
	if len(listeners) == 0 {
		return nil, fmt.Errorf("cannot listen on %s: %w", net.JoinHostPort(bindAddress, strconv.Itoa(port)), firstErr)
	}
	return listeners, nil
}

// freeLoopbackPort asks the OS for an unused loopback port for ssh's internal
// -L listener. The port is released before ssh binds it; if something grabs it
// in between, ssh exits (ExitOnForwardFailure) and the next start picks a new
// one.
func freeLoopbackPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
