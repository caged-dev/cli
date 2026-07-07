package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
)

// mcpTransport is the remote side of the stdio bridge — satisfied by
// *api.MCPConn. Defined here so the bridge is testable without a live server.
type mcpTransport interface {
	Read(ctx context.Context) ([]byte, error)
	Write(ctx context.Context, data []byte) error
	Close() error
}

// cmdMCP runs an MCP server over stdio, proxying to a sandbox's MCP endpoint.
//
// MCP clients (Claude Desktop, Cursor, etc.) spawn this process locally and
// speak newline-delimited JSON-RPC over stdin/stdout — no WebSocket setup:
//
//	{ "mcpServers": { "caged": { "command": "caged", "args": ["mcp", "<sandbox-id>"] } } }
func cmdMCP(args []string) error {
	if len(args) < 1 || args[0] == "" {
		return fmt.Errorf("usage: caged mcp <sandbox-id>")
	}
	sandboxID := args[0]

	client, err := mustClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sandbox, err := client.GetSandbox(ctx, sandboxID)
	if err != nil {
		return fmt.Errorf("getting sandbox: %w", err)
	}
	if sandbox.Status != "running" {
		return fmt.Errorf("sandbox %s is %s (must be running; try 'caged wake %s')", sandboxID, sandbox.Status, sandboxID)
	}

	conn, err := client.ConnectMCP(ctx, sandboxID)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	// Diagnostics go to stderr — stdout is reserved for the MCP protocol.
	fmt.Fprintf(os.Stderr, "caged mcp: bridging stdio to sandbox %s\n", sandboxID)

	return runMCPBridge(ctx, os.Stdin, os.Stdout, conn)
}

// runMCPBridge pumps newline-delimited JSON-RPC messages between a local
// stdio pair and the remote MCP transport. Returns nil on clean EOF/close.
func runMCPBridge(ctx context.Context, in io.Reader, out io.Writer, conn mcpTransport) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)

	// stdin → remote.
	go func() {
		scanner := bufio.NewScanner(in)
		scanner.Buffer(make([]byte, 64*1024), 4<<20) // allow messages up to 4 MiB
		for scanner.Scan() {
			line := bytes.TrimSpace(scanner.Bytes())
			if len(line) == 0 {
				continue
			}
			if err := conn.Write(ctx, line); err != nil {
				errCh <- fmt.Errorf("forwarding to sandbox: %w", err)
				return
			}
		}
		// EOF: the MCP client closed stdin — normal shutdown.
		errCh <- scanner.Err()
	}()

	// Remote → stdout.
	go func() {
		for {
			data, err := conn.Read(ctx)
			if err != nil {
				// Connection closed (sandbox destroyed, network drop, or our
				// own shutdown after stdin EOF) — treat as session end.
				errCh <- nil
				return
			}
			if _, err := out.Write(append(data, '\n')); err != nil {
				errCh <- fmt.Errorf("writing to stdout: %w", err)
				return
			}
		}
	}()

	err := <-errCh
	cancel() // unblock the other pump
	return err
}
