package log

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Record is a snapshot of one slog record, flattened for JSON transport
// and UI consumption. Attrs is a string->scalar map (numbers and bools
// are preserved as-is; complex values are stringified by slog).
type Record struct {
	Time    time.Time      `json:"time"`
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Attrs   map[string]any `json:"attrs,omitempty"`
}

// Tap is an slog.Handler that keeps the most recent records in a shared
// ring buffer and (optionally) forwards each record to a publisher
// callback. WithAttrs / WithGroup return cloned handlers that share the
// same ring buffer (so all sub-loggers feed the same recent-log view).
type Tap struct {
	core   *tapCore
	groups []string
	attrs  []slog.Attr
}

type tapCore struct {
	mu        sync.Mutex
	buf       []Record
	cap       int
	next      int
	full      bool
	publisher func(Record)
}

// NewTap creates a tap with a ring buffer of the given capacity.
func NewTap(capacity int) *Tap {
	if capacity <= 0 {
		capacity = 500
	}
	return &Tap{core: &tapCore{cap: capacity, buf: make([]Record, capacity)}}
}

// SetPublisher swaps in a callback fired for each new record. Pass nil
// to detach.
func (t *Tap) SetPublisher(fn func(Record)) {
	t.core.mu.Lock()
	t.core.publisher = fn
	t.core.mu.Unlock()
}

// Recent returns the latest records (oldest-first). If filter is non-nil
// only matching records are returned. limit caps the result; <=0 means
// no cap.
func (t *Tap) Recent(filter func(Record) bool, limit int) []Record {
	c := t.core
	c.mu.Lock()
	defer c.mu.Unlock()

	count := c.cap
	start := c.next
	if !c.full {
		count = c.next
		start = 0
	}

	out := make([]Record, 0, count)
	for i := 0; i < count; i++ {
		idx := (start + i) % c.cap
		rec := c.buf[idx]
		if filter != nil && !filter(rec) {
			continue
		}
		out = append(out, rec)
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// slog.Handler implementation -----------------------------------------------

func (t *Tap) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (t *Tap) Handle(_ context.Context, r slog.Record) error {
	rec := Record{
		Time:    r.Time,
		Level:   r.Level.String(),
		Message: r.Message,
		Attrs:   map[string]any{},
	}
	for _, a := range t.attrs {
		flattenAttr("", a, rec.Attrs)
	}
	r.Attrs(func(a slog.Attr) bool {
		flattenAttr("", a, rec.Attrs)
		return true
	})
	if len(rec.Attrs) == 0 {
		rec.Attrs = nil
	}

	c := t.core
	c.mu.Lock()
	c.buf[c.next] = rec
	c.next = (c.next + 1) % c.cap
	if c.next == 0 {
		c.full = true
	}
	publisher := c.publisher
	c.mu.Unlock()

	if publisher != nil {
		publisher(rec)
	}
	return nil
}

func (t *Tap) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *t
	clone.attrs = append(append([]slog.Attr{}, t.attrs...), attrs...)
	return &clone
}

func (t *Tap) WithGroup(name string) slog.Handler {
	clone := *t
	clone.groups = append(append([]string{}, t.groups...), name)
	return &clone
}

func flattenAttr(prefix string, a slog.Attr, dst map[string]any) {
	key := a.Key
	if prefix != "" {
		key = prefix + "." + key
	}
	v := a.Value.Resolve()
	if v.Kind() == slog.KindGroup {
		for _, sub := range v.Group() {
			flattenAttr(key, sub, dst)
		}
		return
	}
	dst[key] = jsonSafeValue(v)
}

// jsonSafeValue normalises an slog.Value into something the JSON event
// stream and the browser can render directly. Without this, error / struct
// values leak through as Go objects and end up as "[object Object]" in the
// UI (errors don't have a JSON marshaler, so encoding/json renders them
// as "{}").
func jsonSafeValue(v slog.Value) any {
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindBool:
		return v.Bool()
	case slog.KindInt64:
		return v.Int64()
	case slog.KindUint64:
		return v.Uint64()
	case slog.KindFloat64:
		return v.Float64()
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindTime:
		return v.Time().Format(time.RFC3339Nano)
	}
	raw := v.Any()
	if raw == nil {
		return nil
	}
	if err, ok := raw.(error); ok {
		return err.Error()
	}
	if s, ok := raw.(fmt.Stringer); ok {
		return s.String()
	}
	switch raw.(type) {
	case string, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return raw
	}
	return fmt.Sprintf("%+v", raw)
}
