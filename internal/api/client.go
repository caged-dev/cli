// Package api provides an HTTP client for the Caged API.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an HTTP client for the Caged API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// defaultRequestTimeout bounds most API requests.
const defaultRequestTimeout = 30 * time.Second

// createRequestTimeout bounds sandbox creation, which can include a repo
// clone, package installs, and agent installs (server allows up to 5 min).
const createRequestTimeout = 6 * time.Minute

// NewClient creates a new API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		// No client-wide timeout: per-request timeouts are applied in do().
		httpClient: &http.Client{},
	}
}

// Sandbox represents a sandbox in API responses.
type Sandbox struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	Template    string  `json:"template"`
	IP          string  `json:"ip"`
	CPUs        int     `json:"cpus"`
	MemoryMB    int     `json:"memory_mb"`
	DiskGB      int     `json:"disk_gb"`
	NetworkMode string  `json:"network_mode"`
	CreatedAt   string  `json:"created_at"`
	Budget      float64 `json:"budget,omitempty"`
	Spent       float64 `json:"spent,omitempty"`
}

// CreateSandboxRequest is the request body for creating a sandbox.
type CreateSandboxRequest struct {
	Template    string            `json:"template"`
	CPUs        int               `json:"cpus,omitempty"`
	MemoryMB    int               `json:"memory_mb,omitempty"`
	DiskGB      int               `json:"disk_gb,omitempty"`
	NetworkMode string            `json:"network_mode,omitempty"`
	Allowlist   []string          `json:"allowlist,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Repo        string            `json:"repo,omitempty"`
	RepoToken   string            `json:"repo_token,omitempty"`  // PAT/OAuth token for private repos
	RepoBranch  string            `json:"repo_branch,omitempty"` // Branch to checkout
	RepoCommit  string            `json:"repo_commit,omitempty"` // Specific commit SHA
	RepoSubdir  string            `json:"repo_subdir,omitempty"` // Monorepo subdirectory
	Budget      float64           `json:"budget,omitempty"`      // Budget in USD
	Packages    []string          `json:"packages,omitempty"`    // Pre-install packages
	Agents      []string          `json:"agents,omitempty"`      // AI agents to install
}

// APIError represents a structured API error response.
type APIError struct {
	Status int    `json:"status"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

func (e *APIError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s", e.Title, e.Detail)
	}
	return e.Title
}

// CreateSandbox creates a new sandbox.
func (c *Client) CreateSandbox(ctx context.Context, req *CreateSandboxRequest) (*Sandbox, error) {
	ctx, cancel := context.WithTimeout(ctx, createRequestTimeout)
	defer cancel()
	var s Sandbox
	if err := c.do(ctx, http.MethodPost, "/v1/sandboxes", req, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// GetSandbox gets a sandbox by ID.
func (c *Client) GetSandbox(ctx context.Context, id string) (*Sandbox, error) {
	var s Sandbox
	if err := c.do(ctx, http.MethodGet, "/v1/sandboxes/"+id, nil, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// ListSandboxes lists all sandboxes for the authenticated account.
func (c *Client) ListSandboxes(ctx context.Context) ([]Sandbox, error) {
	var sandboxes []Sandbox
	if err := c.do(ctx, http.MethodGet, "/v1/sandboxes", nil, &sandboxes); err != nil {
		return nil, err
	}
	return sandboxes, nil
}

// DestroySandbox destroys a sandbox by ID.
func (c *Client) DestroySandbox(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/v1/sandboxes/"+id, nil, nil)
}

// SleepSandbox pauses a running sandbox.
func (c *Client) SleepSandbox(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, "/v1/sandboxes/"+id+"/pause", nil, nil)
}

// WakeSandbox resumes a sleeping sandbox.
func (c *Client) WakeSandbox(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, "/v1/sandboxes/"+id+"/resume", nil, nil)
}

// ExecRequest is the request body for executing a command.
type ExecRequest struct {
	Command string `json:"command"`
}

// ExecResponse is the response from an exec call.
type ExecResponse struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// Exec sends a command to a sandbox and returns its output and exit code.
// A non-zero exit code is not an error; err is only set for transport or
// infrastructure failures.
func (c *Client) Exec(ctx context.Context, sandboxID, command string) (string, int, error) {
	var resp ExecResponse
	err := c.do(ctx, http.MethodPost, "/v1/sandboxes/"+sandboxID+"/exec", &ExecRequest{Command: command}, &resp)
	if err != nil {
		return "", 0, err
	}
	if resp.Error != "" {
		return resp.Output, resp.ExitCode, fmt.Errorf("%s", resp.Error)
	}
	return resp.Output, resp.ExitCode, nil
}

// LogEntry represents a single event log entry.
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Message   string `json:"message"`
}

// GetLogs retrieves sandbox event logs.
func (c *Client) GetLogs(ctx context.Context, sandboxID string, follow bool) ([]LogEntry, error) {
	path := "/v1/sandboxes/" + sandboxID + "/logs"
	if follow {
		path += "?follow=true"
	}
	var logs []LogEntry
	if err := c.do(ctx, http.MethodGet, path, nil, &logs); err != nil {
		return nil, err
	}
	return logs, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, result any) error {
	// Apply the default timeout unless the caller already set a deadline.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultRequestTimeout)
		defer cancel()
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "caged-cli/"+Version)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		var apiErr APIError
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Title != "" {
			apiErr.Status = resp.StatusCode
			return &apiErr
		}
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

// Version is set at build time.
var Version = "dev"
