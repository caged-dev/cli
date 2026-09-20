package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// ============================================================================
// A2A Types
// ============================================================================

// A2AAgentRegistration represents a registered A2A agent.
type A2AAgentRegistration struct {
	ID             string     `json:"id"`
	AccountID      string     `json:"account_id"`
	Name           string     `json:"name"`
	Description    string     `json:"description,omitempty"`
	PipelineID     *string    `json:"pipeline_id,omitempty"`
	Template       string     `json:"template,omitempty"`
	Skills         []A2ASkill `json:"skills,omitempty"`
	Public         bool       `json:"public"`
	AllowedOrgs    []string   `json:"allowed_orgs,omitempty"`
	MaxCostPerTask float64    `json:"max_cost_per_task,omitempty"`
	RateLimitRPM   int        `json:"rate_limit_rpm,omitempty"`
	Enabled        bool       `json:"enabled"`
	CreatedAt      string     `json:"created_at"`
	UpdatedAt      string     `json:"updated_at"`
}

// A2ASkill describes a skill/capability an agent provides. It mirrors the
// server's a2a.Skill field for field: `caged a2a agent create -f file.json`
// decodes the user's file into this type and re-encodes it, so a field
// missing here is a field stripped from their registration.
type A2ASkill struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	InputSchema  json.RawMessage   `json:"input_schema,omitempty"`
	OutputSchema json.RawMessage   `json:"output_schema,omitempty"`
	Examples     []A2ASkillExample `json:"examples,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// A2ASkillExample is an example invocation of a skill.
type A2ASkillExample struct {
	Input  json.RawMessage `json:"input"`
	Output json.RawMessage `json:"output,omitempty"`
}

// A2AAgentCard is the discovery document for an A2A agent. It mirrors the
// server's a2a.AgentCard: `caged a2a discover --json` prints what was
// decoded, so a field missing here is a field missing from the user's view
// of the agent.
type A2AAgentCard struct {
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	URL            string         `json:"url"`
	Version        string         `json:"version"`
	Capabilities   []string       `json:"capabilities,omitempty"`
	InputModes     []string       `json:"input_modes,omitempty"`
	OutputModes    []string       `json:"output_modes,omitempty"`
	StreamingMode  string         `json:"streaming_mode,omitempty"`
	Authentication *A2AAuthConfig `json:"authentication,omitempty"`
	Skills         []A2ASkill     `json:"skills,omitempty"`
	Provider       *A2AProvider   `json:"provider,omitempty"`

	// Trust and verification.
	Signature   string `json:"signature,omitempty"`
	PublicKeyID string `json:"public_key_id,omitempty"`
	Verified    bool   `json:"verified,omitempty"`
	ExpiresAt   string `json:"expires_at,omitempty"`

	// Extensions carries the Caged-specific half of the card, including the
	// agent's trust score.
	Extensions *A2AAgentCardExtensions `json:"extensions,omitempty"`
}

// A2AAgentCardExtensions holds the Caged-specific card fields.
type A2AAgentCardExtensions struct {
	CagedAgentID string  `json:"caged_agent_id,omitempty"`
	PipelineID   string  `json:"pipeline_id,omitempty"`
	Template     string  `json:"template,omitempty"`
	TrustScore   int     `json:"trust_score,omitempty"`
	MaxBudget    float64 `json:"max_budget,omitempty"`
	NetworkMode  string  `json:"network_mode,omitempty"`
}

// A2AAuthConfig describes auth requirements for an agent.
type A2AAuthConfig struct {
	Type         string   `json:"type"`
	Schemes      []string `json:"schemes,omitempty"`
	TokenURL     string   `json:"token_url,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
	Instructions string   `json:"instructions,omitempty"`
}

// A2AProvider describes the agent provider.
type A2AProvider struct {
	Name         string `json:"name"`
	Organization string `json:"organization,omitempty"`
	URL          string `json:"url,omitempty"`
	ContactEmail string `json:"contact_email,omitempty"`
}

// A2ATask represents a task delegated to an A2A agent, mirroring the
// server's a2a.Task.
type A2ATask struct {
	ID           string `json:"id"`
	SessionID    string `json:"session_id,omitempty"`
	ParentTaskID string `json:"parent_task_id,omitempty"`

	FromAgentURL string `json:"from_agent_url"`
	ToAgentURL   string `json:"to_agent_url"`
	SkillID      string `json:"skill_id,omitempty"`

	Status        string       `json:"status"`
	StatusMessage string       `json:"status_message,omitempty"`
	Progress      *A2AProgress `json:"progress,omitempty"`

	Messages  []A2AMessage    `json:"messages,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Output    json.RawMessage `json:"output,omitempty"`
	Artifacts []A2AArtifact   `json:"artifacts,omitempty"`

	Priority int            `json:"priority,omitempty"`
	Deadline string         `json:"deadline,omitempty"`
	Budget   *A2ATaskBudget `json:"budget,omitempty"`

	Metadata map[string]string `json:"metadata,omitempty"`
	Tags     []string          `json:"tags,omitempty"`

	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	StartedAt   string `json:"started_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`

	AuthContext *A2AAuthContext `json:"auth_context,omitempty"`
}

// A2ATaskBudget is the resource ceiling recorded on a task.
type A2ATaskBudget struct {
	MaxCostUSD float64 `json:"max_cost_usd,omitempty"`
	// MaxDuration is a Go duration in nanoseconds, matching the server's
	// time.Duration field.
	MaxDuration   time.Duration `json:"max_duration,omitempty"`
	MaxTokens     int           `json:"max_tokens,omitempty"`
	MaxIterations int           `json:"max_iterations,omitempty"`
}

// A2AAuthContext carries the task's authorization context.
type A2AAuthContext struct {
	AgentIdentity string            `json:"agent_identity,omitempty"`
	SessionID     string            `json:"session_id,omitempty"`
	AccountID     string            `json:"account_id,omitempty"`
	Scopes        []string          `json:"scopes,omitempty"`
	Claims        map[string]string `json:"claims,omitempty"`
}

// A2AArtifact is a file or data blob produced by a task.
type A2AArtifact struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	Name      string `json:"name"`
	MimeType  string `json:"mime_type"`
	Size      int64  `json:"size"`
	URI       string `json:"uri"`
	Checksum  string `json:"checksum,omitempty"`
	CreatedAt string `json:"created_at"`
}

// A2AProgress tracks progress on a running task.
type A2AProgress struct {
	Percentage  int    `json:"percentage,omitempty"`
	CurrentStep string `json:"current_step,omitempty"`
	TotalSteps  int    `json:"total_steps,omitempty"`
	StepNumber  int    `json:"step_number,omitempty"`
	Message     string `json:"message,omitempty"`
}

// A2AMessage is a message in an A2A task conversation.
type A2AMessage struct {
	ID        string            `json:"id"`
	TaskID    string            `json:"task_id"`
	Role      string            `json:"role"` // user, agent, system
	Parts     []A2APart         `json:"parts"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt string            `json:"created_at"`
}

// A2APart is a single piece of content in a message.
type A2APart struct {
	Type     string          `json:"type"` // text, file, data, artifact, image, code
	Text     string          `json:"text,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
	MimeType string          `json:"mime_type,omitempty"`

	// For file and artifact parts.
	Name     string `json:"name,omitempty"`
	URI      string `json:"uri,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Checksum string `json:"checksum,omitempty"`

	// For code parts.
	Language string `json:"language,omitempty"`

	// Inline content, base64 for binary.
	Content  string `json:"content,omitempty"`
	Encoding string `json:"encoding,omitempty"`
}

// ============================================================================
// A2A Request/Response Types
// ============================================================================

// CreateA2AAgentRequest is the request body for creating an A2A agent.
type CreateA2AAgentRequest struct {
	Name           string     `json:"name"`
	Description    string     `json:"description,omitempty"`
	PipelineID     string     `json:"pipeline_id,omitempty"`
	Template       string     `json:"template,omitempty"`
	Skills         []A2ASkill `json:"skills,omitempty"`
	Public         bool       `json:"public,omitempty"`
	AllowedOrgs    []string   `json:"allowed_orgs,omitempty"`
	MaxCostPerTask float64    `json:"max_cost_per_task,omitempty"`
	RateLimitRPM   int        `json:"rate_limit_rpm,omitempty"`
}

// UpdateA2AAgentRequest is the request body for updating an A2A agent.
type UpdateA2AAgentRequest struct {
	Name           *string    `json:"name,omitempty"`
	Description    *string    `json:"description,omitempty"`
	Template       *string    `json:"template,omitempty"`
	Skills         []A2ASkill `json:"skills,omitempty"`
	Public         *bool      `json:"public,omitempty"`
	Enabled        *bool      `json:"enabled,omitempty"`
	MaxCostPerTask *float64   `json:"max_cost_per_task,omitempty"`
	RateLimitRPM   *int       `json:"rate_limit_rpm,omitempty"`
}

// CreateA2ATaskRequest is the request body for creating an A2A task.
type CreateA2ATaskRequest struct {
	SessionID string          `json:"session_id,omitempty"`
	SkillID   string          `json:"skill_id,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Priority  int             `json:"priority,omitempty"`
	Streaming bool            `json:"streaming,omitempty"`
}

// SendA2AMessageRequest is the request body for sending a message.
type SendA2AMessageRequest struct {
	Parts []A2APart `json:"parts"`
}

// CancelA2ATaskRequest is the request body for canceling a task.
type CancelA2ATaskRequest struct {
	Reason string `json:"reason,omitempty"`
}

// ============================================================================
// A2A Agent Registration API Methods
// ============================================================================

// ListA2AAgents lists all A2A agent registrations for the account.
func (c *Client) ListA2AAgents(ctx context.Context) ([]A2AAgentRegistration, error) {
	var agents []A2AAgentRegistration
	if err := c.do(ctx, http.MethodGet, "/v1/a2a/agents", nil, &agents); err != nil {
		return nil, err
	}
	return agents, nil
}

// GetA2AAgent gets an A2A agent registration by ID.
func (c *Client) GetA2AAgent(ctx context.Context, id string) (*A2AAgentRegistration, error) {
	var agent A2AAgentRegistration
	if err := c.do(ctx, http.MethodGet, "/v1/a2a/agents/"+id, nil, &agent); err != nil {
		return nil, err
	}
	return &agent, nil
}

// CreateA2AAgent creates a new A2A agent registration.
func (c *Client) CreateA2AAgent(ctx context.Context, req *CreateA2AAgentRequest) (*A2AAgentRegistration, error) {
	var agent A2AAgentRegistration
	if err := c.do(ctx, http.MethodPost, "/v1/a2a/agents", req, &agent); err != nil {
		return nil, err
	}
	return &agent, nil
}

// UpdateA2AAgent updates an A2A agent registration.
func (c *Client) UpdateA2AAgent(ctx context.Context, id string, req *UpdateA2AAgentRequest) (*A2AAgentRegistration, error) {
	var agent A2AAgentRegistration
	if err := c.do(ctx, http.MethodPut, "/v1/a2a/agents/"+id, req, &agent); err != nil {
		return nil, err
	}
	return &agent, nil
}

// DeleteA2AAgent deletes an A2A agent registration.
func (c *Client) DeleteA2AAgent(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/v1/a2a/agents/"+id, nil, nil)
}

// ============================================================================
// A2A Discovery API Methods
// ============================================================================

// DiscoverA2AAgent fetches the Agent Card from a remote A2A agent.
func (c *Client) DiscoverA2AAgent(ctx context.Context, agentURL string) (*A2AAgentCard, error) {
	// Create a separate HTTP client for external requests.
	client := &http.Client{Timeout: 10 * time.Second}

	url := agentURL + "/.well-known/agent.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Caged-CLI/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			Status: resp.StatusCode,
			Title:  "Discovery failed",
			Detail: "Failed to fetch agent card from " + url,
		}
	}

	var card A2AAgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, err
	}

	return &card, nil
}

// ============================================================================
// A2A Task API Methods
// ============================================================================

// CreateA2ATask creates a new task on a remote A2A agent.
func (c *Client) CreateA2ATask(ctx context.Context, agentURL string, req *CreateA2ATaskRequest) (*A2ATask, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := agentURL + "/tasks"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "Caged-CLI/1.0")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, &APIError{
			Status: resp.StatusCode,
			Title:  "Create task failed",
		}
	}

	var task A2ATask
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, err
	}

	return &task, nil
}

// GetA2ATask gets a task from a remote A2A agent.
func (c *Client) GetA2ATask(ctx context.Context, agentURL, taskID string) (*A2ATask, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	url := agentURL + "/tasks/" + taskID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			Status: resp.StatusCode,
			Title:  "Get task failed",
		}
	}

	var task A2ATask
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, err
	}

	return &task, nil
}

// WaitA2ATask polls a task until it completes.
func (c *Client) WaitA2ATask(ctx context.Context, agentURL, taskID string) (*A2ATask, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			task, err := c.GetA2ATask(ctx, agentURL, taskID)
			if err != nil {
				return nil, err
			}

			switch task.Status {
			case "completed", "failed", "canceled":
				return task, nil
			}
		}
	}
}

// SendA2AMessage sends a message to a task on a remote A2A agent.
func (c *Client) SendA2AMessage(ctx context.Context, agentURL, taskID string, req *SendA2AMessageRequest) (*A2AMessage, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := agentURL + "/tasks/" + taskID + "/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, &APIError{
			Status: resp.StatusCode,
			Title:  "Send message failed",
		}
	}

	var msg A2AMessage
	if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

// CancelA2ATask cancels a task on a remote A2A agent.
func (c *Client) CancelA2ATask(ctx context.Context, agentURL, taskID string, req *CancelA2ATaskRequest) error {
	client := &http.Client{Timeout: 10 * time.Second}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	url := agentURL + "/tasks/" + taskID + "/cancel"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return &APIError{
			Status: resp.StatusCode,
			Title:  "Cancel task failed",
		}
	}

	return nil
}
