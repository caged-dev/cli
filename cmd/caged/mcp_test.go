package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeTransport is an in-memory mcpTransport for bridge tests.
type fakeTransport struct {
	mu       sync.Mutex
	received [][]byte    // messages written by the bridge (stdin → remote)
	incoming chan []byte // messages the "server" sends (remote → stdout)
	closed   bool
}

func newFakeTransport() *fakeTransport {
	return &fakeTransport{incoming: make(chan []byte, 16)}
}

func (f *fakeTransport) Read(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg, ok := <-f.incoming:
		if !ok {
			return nil, context.Canceled
		}
		return msg, nil
	}
}

func (f *fakeTransport) Write(_ context.Context, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	f.received = append(f.received, cp)
	return nil
}

func (f *fakeTransport) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

// syncBuffer is a goroutine-safe bytes.Buffer.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestRunMCPBridge_ForwardsStdinToRemote(t *testing.T) {
	ft := newFakeTransport()
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n" +
		"\n" + // blank lines are skipped
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n")
	out := &syncBuffer{}

	err := runMCPBridge(context.Background(), in, out, ft)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ft.mu.Lock()
	defer ft.mu.Unlock()
	if len(ft.received) != 2 {
		t.Fatalf("expected 2 forwarded messages, got %d", len(ft.received))
	}
	if !strings.Contains(string(ft.received[0]), "initialize") {
		t.Errorf("first message wrong: %s", ft.received[0])
	}
	if !strings.Contains(string(ft.received[1]), "tools/list") {
		t.Errorf("second message wrong: %s", ft.received[1])
	}
}

func TestRunMCPBridge_ForwardsRemoteToStdout(t *testing.T) {
	ft := newFakeTransport()
	ft.incoming <- []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`)

	// stdin that blocks until we cancel via remote close.
	inR, inW := newBlockingReader()
	defer inW.close()
	out := &syncBuffer{}

	done := make(chan error, 1)
	go func() { done <- runMCPBridge(context.Background(), inR, out, ft) }()

	// Wait for the response to be written to stdout.
	deadline := time.After(2 * time.Second)
	for !strings.Contains(out.String(), `"id":1`) {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for remote message on stdout")
		case <-time.After(10 * time.Millisecond):
		}
	}
	if !strings.HasSuffix(out.String(), "\n") {
		t.Error("stdout messages must be newline-delimited")
	}

	// Closing stdin ends the bridge cleanly.
	inW.close()
	if err := <-done; err != nil {
		t.Fatalf("unexpected bridge error: %v", err)
	}
}

func TestRunMCPBridge_LargeMessage(t *testing.T) {
	ft := newFakeTransport()
	// A 1 MiB single-line message must pass through (scanner buffer is 4 MiB).
	big := `{"jsonrpc":"2.0","id":1,"params":{"content":"` + strings.Repeat("a", 1<<20) + `"}}`
	in := strings.NewReader(big + "\n")
	out := &syncBuffer{}

	if err := runMCPBridge(context.Background(), in, out, ft); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ft.mu.Lock()
	defer ft.mu.Unlock()
	if len(ft.received) != 1 || len(ft.received[0]) != len(big) {
		t.Fatalf("large message not forwarded intact")
	}
}

// blockingReader blocks Read until closed.
type blockingReader struct {
	ch     chan struct{}
	closer *blockingCloser
}

type blockingCloser struct {
	once sync.Once
	ch   chan struct{}
}

func newBlockingReader() (*blockingReader, *blockingCloser) {
	ch := make(chan struct{})
	c := &blockingCloser{ch: ch}
	return &blockingReader{ch: ch, closer: c}, c
}

func (r *blockingReader) Read(_ []byte) (int, error) {
	<-r.ch
	return 0, io.EOF
}

func (c *blockingCloser) close() {
	c.once.Do(func() { close(c.ch) })
}
