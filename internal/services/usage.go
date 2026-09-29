package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"ssh-tunnel-service/internal/config"
)

// Long-term traffic accounting. Every sample, the bytes a tunnel moved are
// added to three rollups — per minute, per hour and per day — each kept only
// for as long as it is useful. Buckets exist only where there was traffic, so
// an idle tunnel costs nothing. The book is persisted to <home>/data/traffic.json
// so totals and history survive restarts.
const (
	usageMinuteWindow = 24 * time.Hour
	usageHourWindow   = 32 * 24 * time.Hour
	usageDayWindow    = 400 * 24 * time.Hour

	// TrafficSaveInterval is how often a changed traffic book is written out.
	TrafficSaveInterval = time.Minute

	trafficFileVersion = 1
)

// usageBucket is the traffic of one time slot. It is (de)serialised as a
// compact [start, up, down] triple; start is unix seconds.
type usageBucket struct {
	Start int64
	Up    uint64
	Down  uint64
}

func (b usageBucket) MarshalJSON() ([]byte, error) {
	return json.Marshal([3]uint64{uint64(b.Start), b.Up, b.Down})
}

func (b *usageBucket) UnmarshalJSON(data []byte) error {
	var v [3]uint64
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	b.Start, b.Up, b.Down = int64(v[0]), v[1], v[2]
	return nil
}

type usageSeries []usageBucket

// add accounts bytes to the bucket starting at start (merging into the newest
// bucket if the clock went backwards) and drops buckets older than cutoff.
func (s usageSeries) add(start int64, up, down uint64, cutoff int64) usageSeries {
	if n := len(s); n > 0 && s[n-1].Start >= start {
		s[n-1].Up += up
		s[n-1].Down += down
	} else {
		s = append(s, usageBucket{Start: start, Up: up, Down: down})
	}
	i := 0
	for i < len(s) && s[i].Start < cutoff {
		i++
	}
	if i > 0 {
		s = append(usageSeries(nil), s[i:]...)
	}
	return s
}

// since sums the buckets starting at or after from.
func (s usageSeries) since(from int64) UsageTotals {
	var t UsageTotals
	for _, b := range s {
		if b.Start >= from {
			t.UpBytes += b.Up
			t.DownBytes += b.Down
		}
	}
	return t
}

func (s usageSeries) from(from int64) usageSeries {
	out := usageSeries{}
	for _, b := range s {
		if b.Start >= from {
			out = append(out, b)
		}
	}
	return out
}

// usageRollup is one tunnel's (or the aggregate's) long-term history.
type usageRollup struct {
	Minutes usageSeries `json:"minutes"`
	Hours   usageSeries `json:"hours"`
	Days    usageSeries `json:"days"`
}

func (u *usageRollup) add(now time.Time, up, down uint64) {
	if up == 0 && down == 0 {
		return
	}
	u.Minutes = u.Minutes.add(now.Truncate(time.Minute).Unix(), up, down, now.Add(-usageMinuteWindow).Unix())
	u.Hours = u.Hours.add(hourStart(now).Unix(), up, down, now.Add(-usageHourWindow).Unix())
	u.Days = u.Days.add(dayStart(now).Unix(), up, down, now.Add(-usageDayWindow).Unix())
}

func (u usageRollup) clone() usageRollup {
	return usageRollup{
		Minutes: append(usageSeries(nil), u.Minutes...),
		Hours:   append(usageSeries(nil), u.Hours...),
		Days:    append(usageSeries(nil), u.Days...),
	}
}

// Hour and day boundaries follow the service machine's local time, so "today"
// means what the user expects.
func hourStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
}

func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func monthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// UsageTotals is the traffic of a period.
type UsageTotals struct {
	UpBytes   uint64 `json:"up_bytes"`
	DownBytes uint64 `json:"down_bytes"`
}

// TunnelUsage summarises a tunnel's (or the aggregate's) traffic by period.
// All is cumulative since the tunnel was first metered.
type TunnelUsage struct {
	Today UsageTotals `json:"today"`
	Month UsageTotals `json:"month"`
	All   UsageTotals `json:"all"`
}

// UsageSummary is the per-period traffic of every metered tunnel.
type UsageSummary struct {
	At      time.Time              `json:"at"`
	Total   TunnelUsage            `json:"total"`
	Tunnels map[string]TunnelUsage `json:"tunnels"`
}

// UsageRange names a history window and the bucket size that serves it.
type UsageRange string

const (
	UsageRangeDay   UsageRange = "24h"
	UsageRangeMonth UsageRange = "30d"
	UsageRangeYear  UsageRange = "1y"
)

// UsageHistory is one tunnel's (or the aggregate's) traffic over a range, as
// [start, up, down] buckets of StepSeconds; slots without traffic are omitted.
type UsageHistory struct {
	Range       UsageRange  `json:"range"`
	StepSeconds int64       `json:"step_seconds"`
	From        int64       `json:"from"`
	To          int64       `json:"to"`
	Points      usageSeries `json:"points"`
}

// ErrUnknownRange is returned for a history range other than 24h, 30d or 1y.
var ErrUnknownRange = errors.New("range must be 24h, 30d or 1y")

// TrafficUsage returns today's, this month's and all-time traffic per tunnel
// and in aggregate.
func (rt *Runtime) TrafficUsage(now time.Time) UsageSummary {
	tb := rt.traffic
	tb.mu.Lock()
	defer tb.mu.Unlock()
	today, month := dayStart(now).Unix(), monthStart(now).Unix()
	summary := UsageSummary{
		At: now,
		Total: TunnelUsage{
			Today: tb.totalUsage.Days.since(today),
			Month: tb.totalUsage.Days.since(month),
			All:   UsageTotals{UpBytes: tb.retired.UpBytes, DownBytes: tb.retired.DownBytes},
		},
		Tunnels: make(map[string]TunnelUsage, len(tb.entries)),
	}
	for name, e := range tb.entries {
		// Use the last sampled totals, not the live meter, so All agrees with
		// Today/Month (which only grow when a sample is accounted).
		all := UsageTotals{UpBytes: e.lastUp, DownBytes: e.lastDown}
		summary.Tunnels[name] = TunnelUsage{Today: e.usage.Days.since(today), Month: e.usage.Days.since(month), All: all}
		summary.Total.All.UpBytes += all.UpBytes
		summary.Total.All.DownBytes += all.DownBytes
	}
	return summary
}

// TrafficHistoryFor returns the history of one tunnel over rng; an empty name
// selects the aggregate of all tunnels (including removed ones).
func (rt *Runtime) TrafficHistoryFor(name string, rng UsageRange, now time.Time) (UsageHistory, error) {
	tb := rt.traffic
	tb.mu.Lock()
	defer tb.mu.Unlock()
	rollup := tb.totalUsage
	if name != "" {
		e, ok := tb.entries[name]
		if !ok {
			rollup = usageRollup{}
		} else {
			rollup = e.usage
		}
	}
	h := UsageHistory{Range: rng, To: now.Unix()}
	switch rng {
	case UsageRangeDay:
		h.StepSeconds, h.From = 60, now.Add(-24*time.Hour).Truncate(time.Minute).Unix()
		h.Points = rollup.Minutes.from(h.From)
	case UsageRangeMonth:
		h.StepSeconds, h.From = 3600, hourStart(now.Add(-30*24*time.Hour)).Unix()
		h.Points = rollup.Hours.from(h.From)
	case UsageRangeYear:
		h.StepSeconds, h.From = 86400, dayStart(now.AddDate(-1, 0, 0)).Unix()
		h.Points = rollup.Days.from(h.From)
	default:
		return UsageHistory{}, ErrUnknownRange
	}
	return h, nil
}

// ── Persistence ────────────────────────────────────────────────────────────

type persistedTunnel struct {
	UpBytes    uint64      `json:"up_bytes"`
	DownBytes  uint64      `json:"down_bytes"`
	TotalConns uint64      `json:"total_conns"`
	Usage      usageRollup `json:"usage"`
}

type trafficFile struct {
	Version int                        `json:"version"`
	SavedAt time.Time                  `json:"saved_at"`
	Tunnels map[string]persistedTunnel `json:"tunnels"`
	Retired persistedTunnel            `json:"retired"`
	Total   usageRollup                `json:"total"`
}

// LoadTraffic restores totals and history saved by SaveTraffic. A missing file
// is not an error. An unreadable one is moved aside to <path>.corrupt (so the
// next save does not destroy it) and reported; the service then starts fresh.
func (rt *Runtime) LoadTraffic(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read traffic file: %w", err)
	}
	var f trafficFile
	if err := json.Unmarshal(data, &f); err != nil || f.Version != trafficFileVersion {
		_ = os.Rename(path, path+".corrupt")
		if err == nil {
			err = fmt.Errorf("unsupported version %d", f.Version)
		}
		return fmt.Errorf("traffic file %s is unreadable (moved to .corrupt): %w", path, err)
	}

	tb := rt.traffic
	tb.mu.Lock()
	defer tb.mu.Unlock()
	for name, p := range f.Tunnels {
		m := &Meter{}
		m.up.Store(p.UpBytes)
		m.down.Store(p.DownBytes)
		m.total.Store(p.TotalConns)
		// lastUp/lastDown start at the restored totals so the first sample does
		// not count the whole history as one second of traffic.
		tb.entries[name] = &trafficEntry{meter: m, lastUp: p.UpBytes, lastDown: p.DownBytes, usage: p.Usage}
	}
	tb.retired = TrafficCounters{UpBytes: f.Retired.UpBytes, DownBytes: f.Retired.DownBytes, TotalConns: f.Retired.TotalConns}
	tb.totalUsage = f.Total
	return nil
}

// PruneTraffic folds the meters of tunnels that no longer exist (removed or
// renamed while the service was down) into the aggregate.
func (rt *Runtime) PruneTraffic(keep map[string]bool) {
	tb := rt.traffic
	tb.mu.Lock()
	defer tb.mu.Unlock()
	for name := range tb.entries {
		if !keep[name] {
			tb.retireLocked(name)
		}
	}
}

// SaveTraffic writes the traffic book to path atomically, but only when it
// changed since the last save (or force is set).
func (rt *Runtime) SaveTraffic(path string, mode os.FileMode, force bool) error {
	tb := rt.traffic
	tb.mu.Lock()
	if !tb.dirty && !force {
		tb.mu.Unlock()
		return nil
	}
	f := trafficFile{
		Version: trafficFileVersion,
		SavedAt: time.Now(),
		Tunnels: make(map[string]persistedTunnel, len(tb.entries)),
		Retired: persistedTunnel{UpBytes: tb.retired.UpBytes, DownBytes: tb.retired.DownBytes, TotalConns: tb.retired.TotalConns},
		Total:   tb.totalUsage.clone(),
	}
	for name, e := range tb.entries {
		f.Tunnels[name] = persistedTunnel{
			UpBytes:    e.meter.up.Load(),
			DownBytes:  e.meter.down.Load(),
			TotalConns: e.meter.total.Load(),
			Usage:      e.usage.clone(),
		}
	}
	tb.dirty = false
	tb.mu.Unlock()

	err := writeTrafficFile(path, f, mode)
	if err != nil {
		tb.mu.Lock()
		tb.dirty = true
		tb.mu.Unlock()
	}
	return err
}

func writeTrafficFile(path string, f trafficFile, mode os.FileMode) error {
	data, err := json.Marshal(f)
	if err != nil {
		return fmt.Errorf("encode traffic file: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("prepare traffic dir: %w", err)
	}
	return config.WriteRaw(path, data, mode)
}

// FlushTraffic takes a final sample (so bytes moved since the last tick are
// accounted) and saves unconditionally. The service calls it on shutdown,
// after every tunnel has stopped.
func (rt *Runtime) FlushTraffic(path string, mode os.FileMode) error {
	rt.sampleTraffic(time.Now())
	return rt.SaveTraffic(path, mode, true)
}

// RunTrafficSaver saves the traffic book every TrafficSaveInterval while it
// has changed, until ctx is cancelled. Failures are reported through onError
// and retried on the next tick.
func (rt *Runtime) RunTrafficSaver(ctx context.Context, path string, mode os.FileMode, onError func(error)) {
	ticker := time.NewTicker(TrafficSaveInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := rt.SaveTraffic(path, mode, false); err != nil && onError != nil {
				onError(err)
			}
		}
	}
}
