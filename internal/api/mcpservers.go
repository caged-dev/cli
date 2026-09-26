package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// The /v1/mcp surface: registering third-party MCP servers, binding them to
// personas, and the two things an operator gets wrong without help.
//
// Every field here mirrors the server's response type, because `--json` prints
// what was decoded: a field missing from a struct in this file is a field
// missing from the user's view. That is the defect PR #100 in the SDK repos was
// about, and it is the same trap here.

// MCPServer is one registered third-party MCP server.
type MCPServer struct {
	ID          string `json:"id"`
	Alias       string `json:"alias"`
	DisplayName string `json:"display_name"`
	Description string `json:"description,omitempty"`
	Transport   string `json:"transport"`
	Endpoint    string `json:"endpoint,omitempty"`
	CatalogueID string `json:"catalogue_id,omitempty"`
	// Verified is false for a server registered from an arbitrary URL rather
	// than from Caged's reviewed catalogue. It is printed, because the word
	// appears in the tool framing the agent's model reads too.
	Verified bool `json:"verified"`
	// AuthKind says WHICH kind of credential is stored, never the value.
	AuthKind         string `json:"auth_kind"`
	ProtocolEra      string `json:"protocol_era,omitempty"`
	ProtocolVersion  string `json:"protocol_version,omitempty"`
	Status           string `json:"status"`
	QuarantineReason string `json:"quarantine_reason,omitempty"`
	// OAuthNextStep is set on an oauth registration that is not authorized
	// yet. It names the flow rather than leaving a silent server a mystery.
	OAuthNextStep string `json:"oauth_next_step,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// MCPTool is one pinned catalogue entry.
type MCPTool struct {
	// Name is the UPSTREAM name; NamespacedName is what an agent calls and
	// what a policy rule matches. Both, because an operator writing a rule
	// needs the second and an operator reading the server's docs has the first.
	Name           string   `json:"name"`
	NamespacedName string   `json:"namespaced_name"`
	Description    string   `json:"description,omitempty"`
	State          string   `json:"state"`
	Flags          []string `json:"flags,omitempty"`
	// DefinitionTokensEstimate is named an estimate because it is one.
	DefinitionTokensEstimate int     `json:"definition_tokens_estimate"`
	FirstSeenAt              string  `json:"first_seen_at,omitempty"`
	LastSeenAt               string  `json:"last_seen_at,omitempty"`
	ApprovedAt               *string `json:"approved_at,omitempty"`
}

// MCPServerDetail is a server plus its catalogue.
type MCPServerDetail struct {
	MCPServer
	Tools []MCPTool `json:"tools"`
}

// MCPBinding is what a subject may see.
type MCPBinding struct {
	ID                   string   `json:"id"`
	ServerID             string   `json:"server_id"`
	SubjectKind          string   `json:"subject_kind"`
	SubjectID            string   `json:"subject_id,omitempty"`
	ToolAllowlist        []string `json:"tool_allowlist"`
	ToolDenylist         []string `json:"tool_denylist"`
	Pinned               bool     `json:"pinned"`
	Enabled              bool     `json:"enabled"`
	ArgumentCeilingBytes int      `json:"argument_ceiling_bytes,omitempty"`
	CreatedAt            string   `json:"created_at"`
}

// MCPPolicyAdvice is what still has to happen before a bound server's tools
// can actually be called.
//
// This type is the whole reason `caged mcp bind` prints more than "ok". A
// brokered tool name matches nothing in Caged's autonomy-tier table, so a bound
// external tool is denied at every tier until a policy rule allows it — and an
// operator who is not told that binds twelve servers and finds every call
// refused.
type MCPPolicyAdvice struct {
	ServerID       string `json:"server_id"`
	Alias          string `json:"alias"`
	PersonaID      string `json:"persona_id,omitempty"`
	Status         string `json:"status"`
	ToolsEvaluated int    `json:"tools_evaluated"`
	ToolsAllowed   int    `json:"tools_allowed"`
	ToolsPaused    int    `json:"tools_paused"`
	ToolsDenied    int    `json:"tools_denied"`
	Remedy         string `json:"remedy,omitempty"`
	RuleID         string `json:"rule_id,omitempty"`
	ToolPattern    string `json:"tool_pattern,omitempty"`
	GrantedRule    bool   `json:"granted_rule_exists"`
	Explanation    string `json:"explanation"`
	AllowEndpoint  string `json:"allow_endpoint,omitempty"`
	DecidedBy      struct {
		Layer      string `json:"layer,omitempty"`
		PolicyID   string `json:"policy_id,omitempty"`
		PolicyName string `json:"policy_name,omitempty"`
		RuleID     string `json:"rule_id,omitempty"`
		ByDefault  bool   `json:"by_default,omitempty"`
		Editable   bool   `json:"editable"`
	} `json:"decided_by,omitempty"`
}

// MCPGrantResult is what writing the allow rule did.
type MCPGrantResult struct {
	PolicyID string `json:"policy_id"`
	RuleID   string `json:"rule_id"`
	// PolicyCreated is true when the persona had no stored policy and one was
	// created as a copy of its tier template plus this rule. Printed, because
	// "Caged created a policy for this persona" is a fact an operator should
	// not learn later.
	PolicyCreated  bool   `json:"policy_created"`
	TierTemplateID string `json:"tier_template_id,omitempty"`
	ToolPattern    string `json:"tool_pattern"`
	AlreadyPresent bool   `json:"already_present"`
}

// MCPBindResult is a created binding plus the advice.
type MCPBindResult struct {
	MCPBinding
	PolicyAdvice      *MCPPolicyAdvice `json:"policy_advice,omitempty"`
	PolicyRuleWritten *MCPGrantResult  `json:"policy_rule_written,omitempty"`
	PolicyRuleError   string           `json:"policy_rule_error,omitempty"`
}

// MCPRefreshReport is what one catalogue refresh did.
type MCPRefreshReport struct {
	ServerID    string   `json:"server_id"`
	Alias       string   `json:"alias"`
	Era         string   `json:"protocol_era,omitempty"`
	Version     string   `json:"protocol_version,omitempty"`
	Added       []string `json:"added"`
	Unchanged   []string `json:"unchanged"`
	Changed     []string `json:"changed"`
	Withdrawn   []string `json:"withdrawn"`
	Quarantined []string `json:"quarantined"`
}

// MCPCatalogueEntry is one server Caged has reviewed.
type MCPCatalogueEntry struct {
	ID           string `json:"id"`
	DisplayName  string `json:"display_name"`
	Description  string `json:"description,omitempty"`
	Endpoint     string `json:"endpoint,omitempty"`
	Transport    string `json:"transport"`
	AuthKind     string `json:"auth_kind"`
	DefaultAlias string `json:"default_alias,omitempty"`
}

// CreateMCPServerRequest registers a server.
//
// Credential is the bearer token and Headers is the header form. Neither is
// ever printed back by this client, and neither is returned by any API read.
type CreateMCPServerRequest struct {
	CatalogueID string            `json:"catalogue_id,omitempty"`
	Alias       string            `json:"alias,omitempty"`
	DisplayName string            `json:"display_name,omitempty"`
	Description string            `json:"description,omitempty"`
	Transport   string            `json:"transport,omitempty"`
	Endpoint    string            `json:"endpoint,omitempty"`
	AuthKind    string            `json:"auth_kind,omitempty"`
	Credential  string            `json:"credential,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// CreateMCPBindingRequest binds a server to a subject.
type CreateMCPBindingRequest struct {
	ServerID             string   `json:"server_id"`
	SubjectKind          string   `json:"subject_kind"`
	SubjectID            string   `json:"subject_id,omitempty"`
	ToolAllowlist        []string `json:"tool_allowlist,omitempty"`
	ToolDenylist         []string `json:"tool_denylist,omitempty"`
	Pinned               bool     `json:"pinned,omitempty"`
	ArgumentCeilingBytes int      `json:"argument_ceiling_bytes,omitempty"`
	// AllowTools asks Caged to write the policy allow rule in the same
	// request. It is opt-in: binding a server and granting its tools are two
	// decisions.
	AllowTools bool `json:"allow_tools,omitempty"`
}

// ListMCPCatalogue returns the servers Caged has reviewed.
func (c *Client) ListMCPCatalogue(ctx context.Context) ([]MCPCatalogueEntry, error) {
	var resp struct {
		Servers []MCPCatalogueEntry `json:"servers"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1/mcp/catalogue", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Servers, nil
}

// CreateMCPServer registers a third-party MCP server.
func (c *Client) CreateMCPServer(ctx context.Context, req *CreateMCPServerRequest) (*MCPServer, error) {
	var out MCPServer
	if err := c.do(ctx, http.MethodPost, "/v1/mcp/servers", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListMCPServers returns the account's registrations.
func (c *Client) ListMCPServers(ctx context.Context) ([]MCPServer, error) {
	var resp struct {
		Servers []MCPServer `json:"servers"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1/mcp/servers", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Servers, nil
}

// GetMCPServer returns one registration and its pinned catalogue.
func (c *Client) GetMCPServer(ctx context.Context, id string) (*MCPServerDetail, error) {
	var out MCPServerDetail
	if err := c.do(ctx, http.MethodGet, "/v1/mcp/servers/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteMCPServer deregisters a server, its catalogue and its bindings.
func (c *Client) DeleteMCPServer(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/v1/mcp/servers/"+url.PathEscape(id), nil, nil)
}

// RefreshMCPServer re-fetches a server's tool catalogue and reports what
// changed. A changed definition is quarantined, not merged.
func (c *Client) RefreshMCPServer(ctx context.Context, id string) (*MCPRefreshReport, error) {
	var out MCPRefreshReport
	if err := c.do(ctx, http.MethodPost,
		"/v1/mcp/servers/"+url.PathEscape(id)+"/refresh", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateMCPBinding makes a server visible to a subject.
func (c *Client) CreateMCPBinding(ctx context.Context, req *CreateMCPBindingRequest) (*MCPBindResult, error) {
	var out MCPBindResult
	if err := c.do(ctx, http.MethodPost, "/v1/mcp/bindings", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListMCPBindings returns the account's bindings.
func (c *Client) ListMCPBindings(ctx context.Context) ([]MCPBinding, error) {
	var resp struct {
		Bindings []MCPBinding `json:"bindings"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1/mcp/bindings", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Bindings, nil
}

// DeleteMCPBinding removes a binding.
func (c *Client) DeleteMCPBinding(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/v1/mcp/bindings/"+url.PathEscape(id), nil, nil)
}

// MCPReadiness answers, for every server bound to a persona, whether policy
// would actually allow its tools.
func (c *Client) MCPReadiness(ctx context.Context, personaID string) ([]MCPPolicyAdvice, error) {
	path := "/v1/mcp/readiness"
	if personaID != "" {
		path += "?persona_id=" + url.QueryEscape(personaID)
	}
	var resp struct {
		PersonaID string            `json:"persona_id"`
		Servers   []MCPPolicyAdvice `json:"servers"`
		Note      string            `json:"note"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Servers, nil
}

// AllowMCPServer writes the one policy rule that makes a server's tools
// callable by a persona.
func (c *Client) AllowMCPServer(ctx context.Context, serverID, personaID string) (*MCPGrantResult, error) {
	if personaID == "" {
		// Refused here rather than sent, because the server's refusal is a
		// 400 and the reason is worth stating in the user's own terminal:
		// Caged's account policy layer can only restrict, never grant.
		return nil, fmt.Errorf("a persona is required: an allow rule for an external MCP server lives on a " +
			"persona's policy, because the account layer can only restrict and never grant")
	}
	var out MCPGrantResult
	body := map[string]string{"persona_id": personaID}
	if err := c.do(ctx, http.MethodPost,
		"/v1/mcp/servers/"+url.PathEscape(serverID)+"/allow", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DisallowMCPServer removes the rule Caged wrote. It never removes a rule the
// operator wrote themselves.
func (c *Client) DisallowMCPServer(ctx context.Context, serverID, personaID string) error {
	if personaID == "" {
		return fmt.Errorf("a persona is required")
	}
	return c.do(ctx, http.MethodDelete,
		"/v1/mcp/servers/"+url.PathEscape(serverID)+"/allow?persona_id="+url.QueryEscape(personaID), nil, nil)
}

// MCPToolDiff is the review a quarantined definition requires.
type MCPToolDiff struct {
	Alias             string           `json:"alias"`
	ToolName          string           `json:"tool_name"`
	NamespacedName    string           `json:"namespaced_name"`
	State             string           `json:"state"`
	Approved          *MCPToolRevision `json:"approved,omitempty"`
	Current           *MCPToolRevision `json:"current,omitempty"`
	Changed           []string         `json:"changed"`
	AddedProperties   []string         `json:"added_properties"`
	RemovedProperties []string         `json:"removed_properties"`
	ApprovedDigest    string           `json:"approved_digest,omitempty"`
	CurrentDigest     string           `json:"current_digest,omitempty"`
	Explanation       string           `json:"explanation"`
}

// MCPToolRevision is one definition a server advertised.
type MCPToolRevision struct {
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"input_schema,omitempty"`
	Flags        []string        `json:"flags,omitempty"`
	Decision     string          `json:"decision"`
	Note         string          `json:"note,omitempty"`
	FirstSeenAt  string          `json:"first_seen_at,omitempty"`
	LastSeenAt   string          `json:"last_seen_at,omitempty"`
	TokensEstim8 int             `json:"definition_tokens_estimate,omitempty"`
}

// MCPToolDiffFor returns the approved and current definitions of one tool.
func (c *Client) MCPToolDiffFor(ctx context.Context, serverID, tool string) (*MCPToolDiff, error) {
	var out MCPToolDiff
	path := "/v1/mcp/servers/" + url.PathEscape(serverID) + "/tools/" + url.PathEscape(tool) + "/diff"
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ApproveMCPTool releases a pending or quarantined tool.
func (c *Client) ApproveMCPTool(ctx context.Context, serverID, tool string) error {
	path := "/v1/mcp/servers/" + url.PathEscape(serverID) + "/tools/" + url.PathEscape(tool) + "/approve"
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

// RejectMCPTool records a refusal against this exact definition.
func (c *Client) RejectMCPTool(ctx context.Context, serverID, tool, note string) error {
	path := "/v1/mcp/servers/" + url.PathEscape(serverID) + "/tools/" + url.PathEscape(tool) + "/reject"
	return c.do(ctx, http.MethodPost, path, map[string]string{"note": note}, nil)
}

// MCPOAuthState is what a read of the OAuth surface says.
//
// There is no field for a token and there never will be: Caged never returns
// one, and a type with no field for it cannot print one by accident.
type MCPOAuthState struct {
	Status struct {
		ServerID   string   `json:"server_id"`
		Authorized bool     `json:"authorized"`
		Issuer     string   `json:"issuer,omitempty"`
		Scopes     []string `json:"scopes,omitempty"`
		ExpiresAt  string   `json:"expires_at,omitempty"`
		HasRefresh bool     `json:"has_refresh_token"`
		ObtainedAt string   `json:"obtained_at,omitempty"`
		Expired    bool     `json:"expired"`
	} `json:"status"`
	Consents []struct {
		ID        string   `json:"id"`
		PersonaID string   `json:"persona_id,omitempty"`
		Issuer    string   `json:"issuer"`
		Scopes    []string `json:"scopes"`
		GrantedAt string   `json:"granted_at"`
		RevokedAt *string  `json:"revoked_at,omitempty"`
	} `json:"consents"`
	Prospect *struct {
		Issuer                string   `json:"issuer"`
		AuthorizationEndpoint string   `json:"authorization_endpoint"`
		TokenEndpoint         string   `json:"token_endpoint"`
		ResourceName          string   `json:"resource_name,omitempty"`
		Resource              string   `json:"resource"`
		Scopes                []string `json:"scopes"`
		ClientIDMetadata      bool     `json:"client_id_metadata_document_supported"`
		ConsentStatement      string   `json:"consent_statement"`
	} `json:"prospect,omitempty"`
	DiscoveryError string `json:"discovery_error,omitempty"`
}

// GetMCPOAuth reads what is authorized and what authorizing would involve.
func (c *Client) GetMCPOAuth(ctx context.Context, serverID string) (*MCPOAuthState, error) {
	var out MCPOAuthState
	if err := c.do(ctx, http.MethodGet,
		"/v1/mcp/servers/"+url.PathEscape(serverID)+"/oauth", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RecordMCPOAuthConsent records a human's decision. Nothing is forwarded to the
// third party by it — that is the next step, and it is separate on purpose.
func (c *Client) RecordMCPOAuthConsent(ctx context.Context, serverID, personaID, issuer string,
	scopes []string) error {

	body := map[string]any{"approve": true, "issuer": issuer, "scopes": scopes}
	if personaID != "" {
		body["persona_id"] = personaID
	}
	return c.do(ctx, http.MethodPost,
		"/v1/mcp/servers/"+url.PathEscape(serverID)+"/oauth/consent", body, nil)
}

// StartMCPOAuth returns the authorization URL to open. It requires a recorded
// consent, and the server refuses with 409 if there is none.
func (c *Client) StartMCPOAuth(ctx context.Context, serverID, personaID string) (string, error) {
	body := map[string]any{}
	if personaID != "" {
		body["persona_id"] = personaID
	}
	var out struct {
		URL       string `json:"authorization_url"`
		ExpiresIn int    `json:"expires_in_seconds"`
		Note      string `json:"note"`
	}
	if err := c.do(ctx, http.MethodPost,
		"/v1/mcp/servers/"+url.PathEscape(serverID)+"/oauth/authorize", body, &out); err != nil {
		return "", err
	}
	return out.URL, nil
}

// MCPInputRound is a question a third-party server asked, waiting on a person.
type MCPInputRound struct {
	ID         string `json:"id"`
	SandboxID  string `json:"sandbox_id"`
	Alias      string `json:"alias"`
	Tool       string `json:"tool"`
	Round      int    `json:"round"`
	RoundLimit int    `json:"round_limit"`
	State      string `json:"state"`
	Questions  []struct {
		ID      string          `json:"id,omitempty"`
		Kind    string          `json:"kind"`
		Message string          `json:"message"`
		Schema  json.RawMessage `json:"schema,omitempty"`
	} `json:"questions"`
	ApprovalID string `json:"approval_id,omitempty"`
	CreatedAt  string `json:"created_at"`
	ExpiresAt  string `json:"expires_at"`
}

// ListMCPInputs returns the questions waiting on a person.
func (c *Client) ListMCPInputs(ctx context.Context) ([]MCPInputRound, error) {
	var resp struct {
		Inputs []MCPInputRound `json:"inputs"`
		Note   string          `json:"note"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1/mcp/inputs", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Inputs, nil
}

// RespondToMCPInput answers a server's question, or declines it.
func (c *Client) RespondToMCPInput(ctx context.Context, id string, answers []MCPInputAnswer,
	decline bool, note string) error {

	body := map[string]any{"note": note}
	if decline {
		body["decline"] = true
	} else {
		body["answers"] = answers
	}
	return c.do(ctx, http.MethodPost,
		"/v1/mcp/inputs/"+url.PathEscape(id)+"/respond", body, nil)
}

// MCPInputAnswer is one answer to one question.
type MCPInputAnswer struct {
	ID      string          `json:"id"`
	Content json.RawMessage `json:"content"`
}
