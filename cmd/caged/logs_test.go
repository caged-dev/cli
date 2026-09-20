package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/caged-dev/cli/internal/api"
)

type fakeLogFetcher struct {
	mu      sync.Mutex
	polls   int
	batches [][]api.LogEntry
	tails   []int
	err     error
}

func (f *fakeLogFetcher) GetLogs(_ context.Context, _ string, tail int) ([]api.LogEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tails = append(f.tails, tail)
	if f.err != nil {
		return nil, f.err
	}
	i := f.polls
	f.polls++
	if i >= len(f.batches) {
		i = len(f.batches) - 1
	}
	return f.batches[i], nil
}

func entry(ts, msg string) api.LogEntry {
	return api.LogEntry{Timestamp: ts, Type: "lifecycle", Message: msg}
}

func TestLogsSince(t *testing.T) {
	a, b, c := entry("t1", "a"), entry("t2", "b"), entry("t3", "c")
	tests := []struct {
		name string
		logs []api.LogEntry
		key  string
		want []api.LogEntry
	}{
		{"no marker returns all", []api.LogEntry{a, b}, "", []api.LogEntry{a, b}},
		{"marker at end returns none", []api.LogEntry{a, b}, logKey(b), nil},
		{"marker mid window returns rest", []api.LogEntry{a, b, c}, logKey(a), []api.LogEntry{b, c}},
		{"marker rotated out returns all", []api.LogEntry{b, c}, logKey(a), []api.LogEntry{b, c}},
		{"empty window", nil, logKey(a), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := logsSince(tt.logs, tt.key)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if logKey(got[i]) != logKey(tt.want[i]) {
					t.Errorf("[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestFollowLogs_PrintsOnlyNewEntries is the guard on -f actually following:
// before this, -f sent a query parameter the API ignores, fetched once and
// exited.
func TestFollowLogs_PrintsOnlyNewEntries(t *testing.T) {
	f := &fakeLogFetcher{batches: [][]api.LogEntry{
		{entry("t1", "boot")},
		{entry("t1", "boot"), entry("t2", "agent installed")},
		{entry("t1", "boot"), entry("t2", "agent installed"), entry("t3", "ready")},
	}}

	ctx, cancel := context.WithCancel(context.Background())
	var out syncBuffer
	done := make(chan error, 1)
	go func() { done <- followLogs(ctx, f, &out, "cage_abc", 50, time.Millisecond) }()

	deadline := time.After(2 * time.Second)
	for !strings.Contains(out.String(), "ready") {
		select {
		case <-deadline:
			cancel()
			t.Fatalf("never saw the third entry; output: %q", out.String())
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("followLogs: %v", err)
	}

	got := out.String()
	for _, want := range []string{"boot", "agent installed", "ready"} {
		if strings.Count(got, want) != 1 {
			t.Errorf("%q printed %d times, want once:\n%s", want, strings.Count(got, want), got)
		}
	}
	if f.tails[0] != 50 {
		t.Errorf("tail = %d, want 50 passed through", f.tails[0])
	}
}

func TestFollowLogs_ReturnsFetchError(t *testing.T) {
	f := &fakeLogFetcher{err: errors.New("sandbox not found")}
	err := followLogs(context.Background(), f, &syncBuffer{}, "cage_gone", 100, time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "sandbox not found") {
		t.Fatalf("err = %v, want it to wrap the fetch failure", err)
	}
}

func TestFollowLogs_StopsOnCanceledContext(t *testing.T) {
	f := &fakeLogFetcher{batches: [][]api.LogEntry{{entry("t1", "boot")}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := followLogs(ctx, f, &syncBuffer{}, "cage_abc", 100, time.Hour); err != nil {
		t.Fatalf("err = %v, want nil on cancel", err)
	}
}

func TestPrintLogEntry(t *testing.T) {
	var buf syncBuffer
	printLogEntry(&buf, entry("2026-09-20T10:00:00Z", "booted"))
	want := "2026-09-20T10:00:00Z [lifecycle] booted\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}
