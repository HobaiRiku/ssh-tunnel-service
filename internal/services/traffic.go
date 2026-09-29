package services

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Traffic sampling cadence and retention. One point per second for five minutes
// is enough for a live sparkline / `top` view while staying a few KB per tunnel.
const (
	TrafficSampleInterval = time.Second
	trafficHistoryLen     = 300
)

// Meter accumulates the traffic of one tunnel. The relay updates it from its
// copy loops, so every field is atomic and lock-free.
//
// Directions are named from the point of view of whoever connects through the
// tunnel, which makes them mean the same thing for -L and -R: "up" is bytes
// flowing towards the forwarded service, "down" is bytes flowing back from it.
type Meter struct {
	up     atomic.Uint64
	down   atomic.Uint64
	active atomic.Int64
	total  atomic.Uint64
}

// TrafficCounters is the live traffic of one tunnel (or of all of them).
// Byte and connection totals are cumulative since the tunnel was first metered
// and persist across service restarts (see usage.go); rates are bytes per
// second over the most recent sample interval.
type TrafficCounters struct {
	UpBytes     uint64  `json:"up_bytes"`
	DownBytes   uint64  `json:"down_bytes"`
	UpRate      float64 `json:"up_rate"`
	DownRate    float64 `json:"down_rate"`
	ActiveConns int64   `json:"active_conns"`
	TotalConns  uint64  `json:"total_conns"`
}

// TrafficPoint is one history sample: the rates and open connections at T
// (unix milliseconds).
type TrafficPoint struct {
	T     int64   `json:"t"`
	Up    float64 `json:"up"`
	Down  float64 `json:"down"`
	Conns int64   `json:"conns"`
}

// TrafficHistory holds the retained samples, oldest first.
type TrafficHistory struct {
	Total   []TrafficPoint            `json:"total"`
	Tunnels map[string][]TrafficPoint `json:"tunnels"`
}

// TrafficSnapshot is the traffic of every metered tunnel at one sample, plus
// the aggregate across them. History is only filled when asked for.
type TrafficSnapshot struct {
	At         time.Time                  `json:"at"`
	IntervalMS int64                      `json:"interval_ms"`
	Total      TrafficCounters            `json:"total"`
	Tunnels    map[string]TrafficCounters `json:"tunnels"`
	History    *TrafficHistory            `json:"history,omitempty"`
}

type trafficEntry struct {
	meter            *Meter
	lastUp, lastDown uint64
	upRate, downRate float64
	history          []TrafficPoint
	usage            usageRollup
}

// trafficBook is the Runtime's traffic state: one entry per metered tunnel,
// the aggregate history, and the subscribers that receive each new sample.
type trafficBook struct {
	mu      sync.Mutex
	entries map[string]*trafficEntry
	// retired carries the byte and connection totals of tunnels that were
	// removed, so the aggregate never goes backwards when one is deleted.
	retired TrafficCounters
	total   []TrafficPoint
	lastAt  time.Time
	subs    map[chan TrafficSnapshot]struct{}
	// totalUsage is the long-term history of all tunnels together, kept on its
	// own so a removed tunnel's past still counts towards it.
	totalUsage usageRollup
	// dirty is set when anything worth persisting changed since the last save.
	dirty bool
}

func newTrafficBook() *trafficBook {
	return &trafficBook{
		entries: map[string]*trafficEntry{},
		subs:    map[chan TrafficSnapshot]struct{}{},
	}
}

// Meter returns the traffic meter for a tunnel, creating it on first use. The
// meter outlives individual ssh processes, so totals survive reconnects.
func (rt *Runtime) Meter(name string) *Meter {
	tb := rt.traffic
	tb.mu.Lock()
	defer tb.mu.Unlock()
	if e, ok := tb.entries[name]; ok {
		return e.meter
	}
	e := &trafficEntry{meter: &Meter{}}
	tb.entries[name] = e
	tb.dirty = true
	return e.meter
}

// TunnelTraffic returns the live counters for a tunnel, or nil when it has
// never been metered (a direct tunnel, or one that has not run yet).
func (rt *Runtime) TunnelTraffic(name string) *TrafficCounters {
	tb := rt.traffic
	tb.mu.Lock()
	defer tb.mu.Unlock()
	e, ok := tb.entries[name]
	if !ok {
		return nil
	}
	c := e.counters()
	return &c
}

// RenameTraffic moves a tunnel's meter and history to its new name so a rename
// does not reset its counters. Any relay still holding the meter keeps counting
// into the same object.
func (rt *Runtime) RenameTraffic(oldName, newName string) {
	if oldName == newName {
		return
	}
	tb := rt.traffic
	tb.mu.Lock()
	defer tb.mu.Unlock()
	if e, ok := tb.entries[oldName]; ok {
		delete(tb.entries, oldName)
		tb.entries[newName] = e
		tb.dirty = true
	}
}

// DropTraffic forgets a removed tunnel's meter, folding its totals into the
// aggregate so the overall byte count stays monotonic.
func (rt *Runtime) DropTraffic(name string) {
	tb := rt.traffic
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.retireLocked(name)
}

func (tb *trafficBook) retireLocked(name string) {
	e, ok := tb.entries[name]
	if !ok {
		return
	}
	c := e.counters()
	tb.retired.UpBytes += c.UpBytes
	tb.retired.DownBytes += c.DownBytes
	tb.retired.TotalConns += c.TotalConns
	delete(tb.entries, name)
	tb.dirty = true
}

// TrafficSnapshot returns the current counters of every metered tunnel and the
// aggregate; withHistory also copies the retained samples.
func (rt *Runtime) TrafficSnapshot(withHistory bool) TrafficSnapshot {
	tb := rt.traffic
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.snapshotLocked(withHistory)
}

// SubscribeTraffic returns a channel that receives a snapshot (without history)
// after every sample. A slow subscriber misses samples rather than stalling the
// sampler. Call the returned func to unsubscribe.
func (rt *Runtime) SubscribeTraffic() (<-chan TrafficSnapshot, func()) {
	tb := rt.traffic
	ch := make(chan TrafficSnapshot, 1)
	tb.mu.Lock()
	tb.subs[ch] = struct{}{}
	tb.mu.Unlock()
	return ch, func() {
		tb.mu.Lock()
		delete(tb.subs, ch)
		tb.mu.Unlock()
	}
}

// RunTrafficSampler samples every meter once per TrafficSampleInterval until
// ctx is cancelled, turning byte deltas into rates and history points.
func (rt *Runtime) RunTrafficSampler(ctx context.Context) {
	ticker := time.NewTicker(TrafficSampleInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			rt.sampleTraffic(now)
		}
	}
}

func (rt *Runtime) sampleTraffic(now time.Time) {
	tb := rt.traffic
	tb.mu.Lock()
	defer tb.mu.Unlock()

	elapsed := TrafficSampleInterval.Seconds()
	if !tb.lastAt.IsZero() {
		elapsed = now.Sub(tb.lastAt).Seconds()
	}
	tb.lastAt = now
	ms := now.UnixMilli()

	var total TrafficPoint
	total.T = ms
	var totalUp, totalDown uint64
	for _, e := range tb.entries {
		up, down := e.meter.up.Load(), e.meter.down.Load()
		dUp, dDown := up-e.lastUp, down-e.lastDown
		e.upRate, e.downRate = 0, 0
		if elapsed > 0 {
			e.upRate = float64(dUp) / elapsed
			e.downRate = float64(dDown) / elapsed
		}
		e.lastUp, e.lastDown = up, down
		if dUp > 0 || dDown > 0 {
			e.usage.add(now, dUp, dDown)
			totalUp += dUp
			totalDown += dDown
			tb.dirty = true
		}
		p := TrafficPoint{T: ms, Up: e.upRate, Down: e.downRate, Conns: e.meter.active.Load()}
		e.history = appendBounded(e.history, p)
		total.Up += p.Up
		total.Down += p.Down
		total.Conns += p.Conns
	}
	tb.total = appendBounded(tb.total, total)
	tb.totalUsage.add(now, totalUp, totalDown)

	if len(tb.subs) == 0 {
		return
	}
	snap := tb.snapshotLocked(false)
	for ch := range tb.subs {
		select {
		case ch <- snap:
		default:
		}
	}
}

func (tb *trafficBook) snapshotLocked(withHistory bool) TrafficSnapshot {
	at := tb.lastAt
	if at.IsZero() {
		at = time.Now()
	}
	snap := TrafficSnapshot{
		At:         at,
		IntervalMS: TrafficSampleInterval.Milliseconds(),
		Total:      tb.retired,
		Tunnels:    make(map[string]TrafficCounters, len(tb.entries)),
	}
	for name, e := range tb.entries {
		c := e.counters()
		snap.Tunnels[name] = c
		snap.Total.UpBytes += c.UpBytes
		snap.Total.DownBytes += c.DownBytes
		snap.Total.UpRate += c.UpRate
		snap.Total.DownRate += c.DownRate
		snap.Total.ActiveConns += c.ActiveConns
		snap.Total.TotalConns += c.TotalConns
	}
	if withHistory {
		h := &TrafficHistory{
			Total:   append([]TrafficPoint(nil), tb.total...),
			Tunnels: make(map[string][]TrafficPoint, len(tb.entries)),
		}
		for name, e := range tb.entries {
			h.Tunnels[name] = append([]TrafficPoint(nil), e.history...)
		}
		snap.History = h
	}
	return snap
}

func (e *trafficEntry) counters() TrafficCounters {
	return TrafficCounters{
		UpBytes:     e.meter.up.Load(),
		DownBytes:   e.meter.down.Load(),
		UpRate:      e.upRate,
		DownRate:    e.downRate,
		ActiveConns: e.meter.active.Load(),
		TotalConns:  e.meter.total.Load(),
	}
}

func appendBounded(h []TrafficPoint, p TrafficPoint) []TrafficPoint {
	if len(h) < trafficHistoryLen {
		return append(h, p)
	}
	copy(h, h[1:])
	h[len(h)-1] = p
	return h
}
