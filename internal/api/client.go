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
	defer func() { _ = resp.Body.Close() }()

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

// ---------- Pipeline API Types ----------

// Pipeline represents a pipeline definition.
type Pipeline struct {
	ID          string            `json:"id"`
	AccountID   string            `json:"account_id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Stages      []StageDefinition `json:"stages"`
	Defaults    StageDefaults     `json:"defaults,omitempty"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

// StageDefinition describes a pipeline stage.
type StageDefinition struct {
	Name        string          `json:"name"`
	Type        string          `json:"type"` // command, approval, gate, eval
	Command     string          `json:"command,omitempty"`
	Template    string          `json:"template,omitempty"`
	Timeout     string          `json:"timeout,omitempty"`
	DependsOn   []string        `json:"depends_on,omitempty"`
	RequireAck  bool            `json:"require_ack,omitempty"`
	Condition   *StageCondition `json:"condition,omitempty"`
	MaxAttempts int             `json:"max_attempts,omitempty"`
}

// StageCondition configures conditional stage execution.
type StageCondition struct {
	OnSuccess bool   `json:"on_success,omitempty"`
	OnFailure bool   `json:"on_failure,omitempty"`
	If        string `json:"if,omitempty"` // Expression
}

// StageDefaults holds default values for stages.
type StageDefaults struct {
	Template    string `json:"template,omitempty"`
	Timeout     string `json:"timeout,omitempty"`
	MaxAttempts int    `json:"max_attempts,omitempty"`
}

// Run represents a pipeline run.
type Run struct {
	ID         string     `json:"id"`
	PipelineID string     `json:"pipeline_id"`
	Status     string     `json:"status"` // pending, running, paused, succeeded, failed, canceled
	Trigger    string     `json:"trigger"`
	Input      RunInput   `json:"input,omitempty"`
	Output     *RunOutput `json:"output,omitempty"`
	StartedAt  string     `json:"started_at,omitempty"`
	EndedAt    string     `json:"ended_at,omitempty"`
	CreatedAt  string     `json:"created_at"`
}

// RunInput is input to a pipeline run.
type RunInput struct {
	Env    map[string]string `json:"env,omitempty"`
	Repo   string            `json:"repo,omitempty"`
	Branch string            `json:"branch,omitempty"`
}

// RunOutput holds outputs from a completed run.
type RunOutput struct {
	State map[string]any `json:"state,omitempty"`
}

// CreatePipelineRequest is the request body for creating a pipeline.
type CreatePipelineRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Stages      []StageDefinition `json:"stages"`
	Defaults    StageDefaults     `json:"defaults,omitempty"`
}

// StartRunRequest is the request body for starting a pipeline run.
type StartRunRequest struct {
	Trigger string            `json:"trigger,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Repo    string            `json:"repo,omitempty"`
	Branch  string            `json:"branch,omitempty"`
}

// ---------- Pipeline API Methods ----------

// CreatePipeline creates a new pipeline.
func (c *Client) CreatePipeline(ctx context.Context, req *CreatePipelineRequest) (*Pipeline, error) {
	var p Pipeline
	if err := c.do(ctx, http.MethodPost, "/v1/pipelines", req, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// ListPipelines lists all pipelines.
func (c *Client) ListPipelines(ctx context.Context) ([]Pipeline, error) {
	var pipelines []Pipeline
	if err := c.do(ctx, http.MethodGet, "/v1/pipelines", nil, &pipelines); err != nil {
		return nil, err
	}
	return pipelines, nil
}

// GetPipeline gets a pipeline by ID.
func (c *Client) GetPipeline(ctx context.Context, id string) (*Pipeline, error) {
	var p Pipeline
	if err := c.do(ctx, http.MethodGet, "/v1/pipelines/"+id, nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// DeletePipeline deletes a pipeline by ID.
func (c *Client) DeletePipeline(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/v1/pipelines/"+id, nil, nil)
}

// StartRun starts a new pipeline run.
func (c *Client) StartRun(ctx context.Context, pipelineID string, req *StartRunRequest) (*Run, error) {
	var r Run
	if err := c.do(ctx, http.MethodPost, "/v1/pipelines/"+pipelineID+"/runs", req, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// ListRuns lists runs for a pipeline.
func (c *Client) ListRuns(ctx context.Context, pipelineID string) ([]Run, error) {
	var runs []Run
	if err := c.do(ctx, http.MethodGet, "/v1/pipelines/"+pipelineID+"/runs", nil, &runs); err != nil {
		return nil, err
	}
	return runs, nil
}

// GetRun gets a run by ID.
func (c *Client) GetRun(ctx context.Context, pipelineID, runID string) (*Run, error) {
	var r Run
	if err := c.do(ctx, http.MethodGet, "/v1/pipelines/"+pipelineID+"/runs/"+runID, nil, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// CancelRun cancels a pipeline run.
func (c *Client) CancelRun(ctx context.Context, pipelineID, runID string) error {
	return c.do(ctx, http.MethodPost, "/v1/pipelines/"+pipelineID+"/runs/"+runID+"/cancel", nil, nil)
}

// ---------- Pipeline State API Types ----------

// StateEntry is a key/value entry in a pipeline run's state store.
type StateEntry struct {
	RunID     string `json:"run_id"`
	Key       string `json:"key"`
	Value     any    `json:"value"`
	Type      string `json:"type,omitempty"`      // string, json, file, patch, artifact
	MimeType  string `json:"mime_type,omitempty"` // For typed artifacts
	SizeBytes int    `json:"size_bytes"`
	ExpiresAt string `json:"expires_at,omitempty"`
	CreatedBy string `json:"created_by"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// SetStateRequest is the request body for setting a state entry.
type SetStateRequest struct {
	Value      any    `json:"value"`
	Type       string `json:"type,omitempty"`
	MimeType   string `json:"mime_type,omitempty"`
	CreatedBy  string `json:"created_by,omitempty"`
	TTLSeconds int    `json:"ttl_seconds,omitempty"`
}

// ---------- Pipeline State API Methods ----------

// ListState lists all state entries for a pipeline run.
func (c *Client) ListState(ctx context.Context, pipelineID, runID string) ([]StateEntry, error) {
	var entries []StateEntry
	if err := c.do(ctx, http.MethodGet, "/v1/pipelines/"+pipelineID+"/runs/"+runID+"/state", nil, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// GetState gets a single state entry by key.
func (c *Client) GetState(ctx context.Context, pipelineID, runID, key string) (*StateEntry, error) {
	var entry StateEntry
	if err := c.do(ctx, http.MethodGet, "/v1/pipelines/"+pipelineID+"/runs/"+runID+"/state/"+key, nil, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// SetState sets a state entry.
func (c *Client) SetState(ctx context.Context, pipelineID, runID, key string, req *SetStateRequest) (*StateEntry, error) {
	var entry StateEntry
	if err := c.do(ctx, http.MethodPut, "/v1/pipelines/"+pipelineID+"/runs/"+runID+"/state/"+key, req, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// DeleteState deletes a state entry.
func (c *Client) DeleteState(ctx context.Context, pipelineID, runID, key string) error {
	return c.do(ctx, http.MethodDelete, "/v1/pipelines/"+pipelineID+"/runs/"+runID+"/state/"+key, nil, nil)
}
