package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/caged-dev/cli/internal/api"
)

// logPollInterval is how often -f asks for the log again. The endpoint is a
// plain GET returning a window of the log — there is no streaming variant —
// so following is polling, and the interval is the floor on how stale the
// tail can look.
const logPollInterval = 2 * time.Second

// logFetcher is the slice of the API client that following needs.
type logFetcher interface {
	GetLogs(ctx context.Context, sandboxID string, tail int) ([]api.LogEntry, error)
}

func cmdLogs(args []string) error {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	follow := fs.Bool("f", false, "Follow log output (polls; Ctrl-C to stop)")
	tail := fs.Int("tail", 100, "Number of trailing entries to fetch")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) < 1 {
		return fmt.Errorf("usage: caged logs [-f] [--tail N] <sandbox-id>")
	}
	sandboxID := remaining[0]

	client, err := mustClient()
	if err != nil {
		return err
	}

	if *follow {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return followLogs(ctx, client, os.Stdout, sandboxID, *tail, logPollInterval)
	}

	logs, err := client.GetLogs(context.Background(), sandboxID, *tail)
	if err != nil {
		return fmt.Errorf("getting logs: %w", err)
	}

	if len(logs) == 0 {
		fmt.Println("No logs yet.")
		return nil
	}

	for _, entry := range logs {
		printLogEntry(os.Stdout, entry)
	}
	return nil
}

// followLogs polls the log window and prints whatever is new since the last
// poll, until ctx is canceled. A poll failure ends the follow rather than
// looping silently on a sandbox that has been destroyed.
func followLogs(ctx context.Context, f logFetcher, w io.Writer, sandboxID string, tail int, interval time.Duration) error {
	var last string // Key of the last entry printed.
	for {
		logs, err := f.GetLogs(ctx, sandboxID, tail)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("getting logs: %w", err)
		}

		for _, entry := range logsSince(logs, last) {
			printLogEntry(w, entry)
		}
		if len(logs) > 0 {
			last = logKey(logs[len(logs)-1])
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
		}
	}
}

// logsSince returns the entries that follow the entry with the given key. An
// empty key, or a key no longer in the window because the sandbox produced
// more than tail entries between polls, yields the whole window: printing a
// line twice is recoverable, dropping one silently is not.
func logsSince(logs []api.LogEntry, lastKey string) []api.LogEntry {
	if lastKey == "" {
		return logs
	}
	for i := len(logs) - 1; i >= 0; i-- {
		if logKey(logs[i]) == lastKey {
			return logs[i+1:]
		}
	}
	return logs
}

// logKey identifies an entry. The API sends no entry ID, so identity is the
// content: two identical lines at the same timestamp are indistinguishable,
// and treating them as one is the safe direction for a tail marker.
func logKey(e api.LogEntry) string {
	return e.Timestamp + "\x00" + e.Type + "\x00" + e.Message
}

func printLogEntry(w io.Writer, e api.LogEntry) {
	fmt.Fprintf(w, "%s [%s] %s\n", e.Timestamp, e.Type, e.Message) //nolint:errcheck // stdout write failure is not actionable here
}
