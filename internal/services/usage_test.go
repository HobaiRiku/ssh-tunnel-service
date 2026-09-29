package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUsageSeriesMergesAndTrims(t *testing.T) {
	var s usageSeries
	s = s.add(100, 1, 2, 0)
	s = s.add(100, 3, 4, 0) // same bucket merges
	s = s.add(90, 5, 5, 0)  // clock went backwards: merge into newest
	s = s.add(200, 7, 0, 150)
	if len(s) != 1 || s[0].Start != 200 || s[0].Up != 7 {
		t.Fatalf("expected only the bucket after the cutoff, got %+v", s)
	}
	var m usageSeries
	m = m.add(100, 1, 2, 0)
	m = m.add(100, 3, 4, 0)
	m = m.add(90, 5, 5, 0)
	if len(m) != 1 || m[0].Up != 9 || m[0].Down != 11 {
		t.Fatalf("merge = %+v", m)
	}
}

func TestSampleTrafficAccountsUsageByPeriod(t *testing.T) {
	rt := NewRuntime()
	m := rt.Meter("db")
	now := time.Date(2026, 9, 15, 10, 30, 0, 0, time.Local)

	rt.sampleTraffic(now)
	m.up.Add(100)
	m.down.Add(1000)
	rt.sampleTraffic(now.Add(time.Second))
	m.down.Add(500)
	rt.sampleTraffic(now.Add(2 * time.Second))

	u := rt.TrafficUsage(now.Add(3 * time.Second))
	db := u.Tunnels["db"]
	if db.Today.DownBytes != 1500 || db.Today.UpBytes != 100 || db.Month.DownBytes != 1500 || db.All.DownBytes != 1500 {
		t.Fatalf("usage = %+v", db)
	}
	if u.Total.Today.DownBytes != 1500 {
		t.Fatalf("total usage = %+v", u.Total)
	}

	h, err := rt.TrafficHistoryFor("db", UsageRangeDay, now.Add(3*time.Second))
	if err != nil || h.StepSeconds != 60 || len(h.Points) != 1 || h.Points[0].Down != 1500 {
		t.Fatalf("24h history = %+v (%v)", h, err)
	}
	if _, err := rt.TrafficHistoryFor("db", "7d", now); err != ErrUnknownRange {
		t.Fatalf("expected ErrUnknownRange, got %v", err)
	}
}

func TestTrafficSurvivesSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "traffic.json")
	now := time.Now()

	rt := NewRuntime()
	m := rt.Meter("db")
	rt.sampleTraffic(now)
	m.up.Add(10)
	m.down.Add(4000)
	m.total.Add(3)
	rt.sampleTraffic(now.Add(time.Second))
	rt.Meter("gone").down.Add(50)
	rt.sampleTraffic(now.Add(2 * time.Second))
	if err := rt.SaveTraffic(path, 0o600, false); err != nil {
		t.Fatalf("SaveTraffic: %v", err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("traffic file mode: %v %v", info, err)
	}

	restored := NewRuntime()
	if err := restored.LoadTraffic(path); err != nil {
		t.Fatalf("LoadTraffic: %v", err)
	}
	restored.PruneTraffic(map[string]bool{"db": true})
	c := restored.TunnelTraffic("db")
	if c == nil || c.DownBytes != 4000 || c.UpBytes != 10 || c.TotalConns != 3 {
		t.Fatalf("restored counters = %+v", c)
	}
	// The first sample after a restart must not turn the history into a spike.
	restored.sampleTraffic(now.Add(3 * time.Second))
	if c := restored.TunnelTraffic("db"); c.DownRate != 0 {
		t.Fatalf("rate after restore = %v, want 0", c.DownRate)
	}
	u := restored.TrafficUsage(now.Add(3 * time.Second))
	if u.Tunnels["db"].Today.DownBytes != 4000 {
		t.Fatalf("restored daily usage = %+v", u.Tunnels["db"])
	}
	// The pruned tunnel's bytes still count in the aggregate.
	if u.Total.All.DownBytes != 4050 || u.Total.Today.DownBytes != 4050 {
		t.Fatalf("aggregate after prune = %+v", u.Total)
	}
	if restored.TunnelTraffic("gone") != nil {
		t.Fatalf("pruned tunnel should have no meter")
	}
}

func TestSaveTrafficSkipsWhenUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traffic.json")
	rt := NewRuntime()
	if err := rt.SaveTraffic(path, 0o600, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("an unchanged book should not be written, stat err = %v", err)
	}
	if err := rt.FlushTraffic(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("flush must always write: %v", err)
	}
}

func TestLoadTrafficMovesCorruptFileAside(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "traffic.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	rt := NewRuntime()
	if err := rt.LoadTraffic(path); err == nil {
		t.Fatal("expected an error for a corrupt file")
	}
	if _, err := os.Stat(path + ".corrupt"); err != nil {
		t.Fatalf("corrupt file should be kept aside: %v", err)
	}
	if err := rt.LoadTraffic(filepath.Join(dir, "missing.json")); err != nil {
		t.Fatalf("a missing file is not an error: %v", err)
	}
}
