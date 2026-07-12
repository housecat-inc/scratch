package testkit

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
)

type LogRecorder struct {
	attrs   []slog.Attr
	mark    *int
	mu      *sync.Mutex
	records *[]map[string]any
}

func NewLogRecorder(t *testing.T, artifacts *Artifacts) *LogRecorder {
	t.Helper()

	recorder := &LogRecorder{
		mark:    new(int),
		mu:      &sync.Mutex{},
		records: &[]map[string]any{},
	}

	t.Cleanup(func() {
		lines := recorder.lines(0)
		if len(lines) == 0 {
			return
		}
		out := strings.Join(lines, "\n") + "\n"
		if err := os.WriteFile(artifacts.ActualPath("logs.jsonl"), []byte(out), 0o644); err != nil {
			t.Error(err)
		}
	})

	return recorder
}

func (r *LogRecorder) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

func (r *LogRecorder) Handle(ctx context.Context, record slog.Record) error {
	entry := map[string]any{
		"level": record.Level.String(),
		"msg":   record.Message,
	}
	for _, attr := range r.attrs {
		entry[attr.Key] = attr.Value.Resolve().Any()
	}
	record.Attrs(func(attr slog.Attr) bool {
		entry[attr.Key] = attr.Value.Resolve().Any()
		return true
	})

	r.mu.Lock()
	defer r.mu.Unlock()
	*r.records = append(*r.records, entry)
	return nil
}

func (r *LogRecorder) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LogRecorder{
		attrs:   append(slices.Clip(r.attrs), attrs...),
		mark:    r.mark,
		mu:      r.mu,
		records: r.records,
	}
}

func (r *LogRecorder) WithGroup(name string) slog.Handler {
	return r
}

func (r *LogRecorder) Count(subset string) int {
	var want map[string]any
	if err := json.Unmarshal([]byte(subset), &want); err != nil {
		panic(err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for _, record := range (*r.records)[*r.mark:] {
		if matchesSubset(record, want) {
			count++
		}
	}
	return count
}

func (r *LogRecorder) Lines() []string {
	r.mu.Lock()
	mark := *r.mark
	r.mu.Unlock()
	return r.lines(mark)
}

func (r *LogRecorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	*r.mark = len(*r.records)
}

func (r *LogRecorder) lines(from int) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := []string{}
	for _, record := range (*r.records)[from:] {
		bytes, err := json.Marshal(record)
		if err != nil {
			panic(err)
		}
		out = append(out, string(bytes))
	}
	return out
}

func matchesSubset(record map[string]any, want map[string]any) bool {
	for key, value := range want {
		got, ok := record[key]
		if !ok {
			return false
		}

		gotJSON, err := json.Marshal(normalizeJSON(got))
		if err != nil {
			return false
		}
		wantJSON, err := json.Marshal(value)
		if err != nil {
			return false
		}
		if string(gotJSON) != string(wantJSON) {
			return false
		}
	}
	return true
}
