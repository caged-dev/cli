package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/coder/websocket"
)

// terminalMessage mirrors the server's WebSocket terminal protocol.
type terminalMessage struct {
	Type string `json:"type"` // "input", "output", "resize", "ping", "pong"
	Data string `json:"data,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
	Cols uint16 `json:"cols,omitempty"`
}

// TerminalConn is a live PTY session attached to a sandbox.
type TerminalConn struct {
	conn *websocket.Conn
}

// ConnectTerminal opens a WebSocket PTY session to a running sandbox.
func (c *Client) ConnectTerminal(ctx context.Context, sandboxID string, rows, cols int) (*TerminalConn, error) {
	wsBase := c.baseURL
	if strings.HasPrefix(wsBase, "https://") {
		wsBase = "wss://" + strings.TrimPrefix(wsBase, "https://")
	} else if strings.HasPrefix(wsBase, "http://") {
		wsBase = "ws://" + strings.TrimPrefix(wsBase, "http://")
	}

	u := fmt.Sprintf("%s/v1/sandboxes/%s/terminal?token=%s&rows=%d&cols=%d",
		wsBase, url.PathEscape(sandboxID), url.QueryEscape(c.apiKey), rows, cols)

	conn, _, err := websocket.Dial(ctx, u, &websocket.DialOptions{
		Subprotocols: []string{"terminal"},
	})
	if err != nil {
		return nil, fmt.Errorf("dialing terminal: %w", err)
	}
	conn.SetReadLimit(1 << 20) // 1MB

	return &TerminalConn{conn: conn}, nil
}

// Read blocks until the next chunk of terminal output arrives.
// Returns an error when the session ends or the connection drops.
func (t *TerminalConn) Read(ctx context.Context) ([]byte, error) {
	for {
		_, data, err := t.conn.Read(ctx)
		if err != nil {
			return nil, err
		}
		var msg terminalMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		if msg.Type == "output" {
			return []byte(msg.Data), nil
		}
		// Ignore pong and other control messages.
	}
}

// WriteInput sends keystrokes to the sandbox PTY.
func (t *TerminalConn) WriteInput(ctx context.Context, data []byte) error {
	msg := terminalMessage{Type: "input", Data: string(data)}
	b, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("encoding input: %w", err)
	}
	return t.conn.Write(ctx, websocket.MessageText, b)
}

// Resize updates the remote PTY dimensions.
func (t *TerminalConn) Resize(ctx context.Context, rows, cols uint16) error {
	msg := terminalMessage{Type: "resize", Rows: rows, Cols: cols}
	b, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("encoding resize: %w", err)
	}
	return t.conn.Write(ctx, websocket.MessageText, b)
}

// Close terminates the terminal session.
func (t *TerminalConn) Close() error {
	return t.conn.Close(websocket.StatusNormalClosure, "client disconnect")
}
