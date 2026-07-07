package api

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/coder/websocket"
)

// MCPConn is a live MCP WebSocket session for a sandbox. Messages are opaque
// JSON-RPC payloads — the connection is a transparent transport bridge.
type MCPConn struct {
	conn *websocket.Conn
}

// maxMCPMessageBytes bounds a single MCP message (matches the server limit).
const maxMCPMessageBytes = 4 << 20 // 4 MiB

// ConnectMCP opens the MCP WebSocket endpoint for a running sandbox.
func (c *Client) ConnectMCP(ctx context.Context, sandboxID string) (*MCPConn, error) {
	wsBase := c.baseURL
	if strings.HasPrefix(wsBase, "https://") {
		wsBase = "wss://" + strings.TrimPrefix(wsBase, "https://")
	} else if strings.HasPrefix(wsBase, "http://") {
		wsBase = "ws://" + strings.TrimPrefix(wsBase, "http://")
	}

	u := fmt.Sprintf("%s/v1/sandboxes/%s/mcp?token=%s",
		wsBase, url.PathEscape(sandboxID), url.QueryEscape(c.apiKey))

	conn, _, err := websocket.Dial(ctx, u, &websocket.DialOptions{
		Subprotocols: []string{"mcp"},
	})
	if err != nil {
		return nil, fmt.Errorf("dialing MCP endpoint: %w", err)
	}
	conn.SetReadLimit(maxMCPMessageBytes)

	return &MCPConn{conn: conn}, nil
}

// Read blocks until the next MCP message arrives from the server.
func (m *MCPConn) Read(ctx context.Context) ([]byte, error) {
	_, data, err := m.conn.Read(ctx)
	return data, err
}

// Write sends an MCP message to the server.
func (m *MCPConn) Write(ctx context.Context, data []byte) error {
	return m.conn.Write(ctx, websocket.MessageText, data)
}

// Close terminates the MCP session.
func (m *MCPConn) Close() error {
	return m.conn.Close(websocket.StatusNormalClosure, "client disconnect")
}
