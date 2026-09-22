package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The properties these assert:
//
//  1. Every field the server sends is decoded. A field missing from a struct in
//     mcpservers.go is a field missing from `--json` and from every printed
//     table — which is the defect the SDK repos' PR #100 was about, and the same
//     trap here.
//  2. No credential is ever sent back or printed. The request types carry one;
//     the response types have no field for one.
//  3. A refusal the server would give as a 400 is refused HERE when the reason
//     is worth stating in the user's own terminal.

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &Client{baseURL: server.URL, apiKey: "k", httpClient: server.Client()}
}

// serverResponse is the full shape cmd/server returns for one registration,
// written out so a field added there and forgotten here fails a test.
const serverResponse = `{
  "id": "0f4c1b2e-0000-0000-0000-000000000001",
  "alias": "github",
  "display_name": "GitHub",
  "description": "the GitHub MCP server",
  "transport": "streamable_http",
  "endpoint": "https://api.githubcopilot.com/mcp",
  "catalogue_id": "io.github.github/github-mcp-server",
  "verified": true,
  "auth_kind": "oauth",
  "protocol_era": "modern",
  "protocol_version": "2026-07-28",
  "status": "active",
  "quarantine_reason": "",
  "oauth_next_step": "GET /v1/mcp/servers/0f4c.../oauth to see the authorization server",
  "created_at": "2026-09-22T10:00:00Z",
  "updated_at": "2026-09-22T10:05:00Z"
}`

func TestMCPServerDecodesEveryField(t *testing.T) {
	var srv MCPServer
	if err := json.Unmarshal([]byte(serverResponse), &srv); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for name, got := range map[string]string{
		"alias":            srv.Alias,
		"display_name":     srv.DisplayName,
		"description":      srv.Description,
		"transport":        srv.Transport,
		"endpoint":         srv.Endpoint,
		"catalogue_id":     srv.CatalogueID,
		"auth_kind":        srv.AuthKind,
		"protocol_era":     srv.ProtocolEra,
		"protocol_version": srv.ProtocolVersion,
		"status":           srv.Status,
		"oauth_next_step":  srv.OAuthNextStep,
		"created_at":       srv.CreatedAt,
		"updated_at":       srv.UpdatedAt,
	} {
		if got == "" {
			t.Fatalf("%s was dropped on decode", name)
		}
	}
	if !srv.Verified {
		t.Fatal("verified was dropped")
	}
}

// The response type must have NO field for a credential. A struct with one
// would print a token on --json the day the server started sending it.
func TestMCPServerResponseHasNoCredentialField(t *testing.T) {
	encoded, err := json.Marshal(MCPServer{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	lower := strings.ToLower(string(encoded))
	for _, forbidden := range []string{"credential", "token", "secret", "auth_sealed", "headers"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("MCPServer carries a %q field: %s", forbidden, encoded)
		}
	}
}

func TestCreateMCPServerSendsTheCredentialAndNothingElseLeaksBack(t *testing.T) {
	var sent CreateMCPServerRequest
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
			t.Fatalf("decoding the request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(serverResponse))
	})
	srv, err := client.CreateMCPServer(context.Background(), &CreateMCPServerRequest{
		Alias: "github", Endpoint: "https://mcp.example.com/mcp",
		AuthKind: "bearer", Credential: "ghp_secret",
	})
	if err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}
	if sent.Credential != "ghp_secret" {
		t.Fatalf("the credential was not sent: %+v", sent)
	}
	encoded, err := json.Marshal(srv)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), "ghp_secret") {
		t.Fatalf("the credential came back in the response type: %s", encoded)
	}
}

const bindResponse = `{
  "id": "5c7e0000-0000-0000-0000-000000000002",
  "server_id": "0f4c1b2e-0000-0000-0000-000000000001",
  "subject_kind": "persona",
  "subject_id": "9a1b0000-0000-0000-0000-000000000003",
  "tool_allowlist": ["get_issue"],
  "tool_denylist": [],
  "pinned": true,
  "enabled": true,
  "argument_ceiling_bytes": 8192,
  "created_at": "2026-09-22T10:00:00Z",
  "policy_advice": {
    "server_id": "0f4c1b2e-0000-0000-0000-000000000001",
    "alias": "github",
    "persona_id": "9a1b0000-0000-0000-0000-000000000003",
    "status": "unclassified",
    "tools_evaluated": 26,
    "tools_allowed": 0,
    "tools_paused": 0,
    "tools_denied": 26,
    "remedy": "allow_mcp_server",
    "rule_id": "mcp:allow:github",
    "tool_pattern": "github__*",
    "granted_rule_exists": false,
    "explanation": "this server is bound but nothing classifies its tools",
    "allow_endpoint": "POST /v1/mcp/servers/0f4c/allow",
    "decided_by": {
      "layer": "autonomy_tier",
      "rule_id": "tool:default-deny",
      "by_default": true,
      "editable": false
    }
  },
  "policy_rule_written": {
    "policy_id": "b71d0000-0000-0000-0000-000000000004",
    "rule_id": "mcp:allow:github",
    "policy_created": true,
    "tier_template_id": "autonomy:trusted",
    "tool_pattern": "github__*",
    "already_present": false
  }
}`

// The advice is the whole reason `caged mcp bind` prints more than "ok", so
// every field of it has to survive the decode.
func TestBindResultDecodesTheWholeAdvice(t *testing.T) {
	var result MCPBindResult
	if err := json.Unmarshal([]byte(bindResponse), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.PolicyAdvice == nil {
		t.Fatal("policy_advice was dropped, which is the fact an operator needs most")
	}
	advice := result.PolicyAdvice
	if advice.Status != "unclassified" || advice.Remedy != "allow_mcp_server" {
		t.Fatalf("advice = %+v", advice)
	}
	if advice.ToolsEvaluated != 26 || advice.ToolsDenied != 26 {
		t.Fatalf("counts = %+v", advice)
	}
	if advice.RuleID != "mcp:allow:github" || advice.ToolPattern != "github__*" {
		t.Fatalf("the rule the operator needs to write was dropped: %+v", advice)
	}
	if advice.Explanation == "" || advice.AllowEndpoint == "" {
		t.Fatalf("advice = %+v", advice)
	}
	if advice.DecidedBy.Layer != "autonomy_tier" || advice.DecidedBy.RuleID != "tool:default-deny" {
		t.Fatalf("decided_by = %+v", advice.DecidedBy)
	}
	if advice.DecidedBy.Editable {
		t.Fatal("editable must decode false: a tier template is code and has no editor")
	}
	if result.PolicyRuleWritten == nil || !result.PolicyRuleWritten.PolicyCreated {
		t.Fatalf("policy_rule_written = %+v", result.PolicyRuleWritten)
	}
	if result.PolicyRuleWritten.TierTemplateID != "autonomy:trusted" {
		t.Fatal("the template the created policy copied was dropped; the operator should be told")
	}
	if result.ArgumentCeilingBytes != 8192 || !result.Pinned {
		t.Fatalf("binding fields lost: %+v", result.MCPBinding)
	}
}

func TestCreateMCPBindingSendsAllowTools(t *testing.T) {
	var sent map[string]any
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
			t.Fatalf("decode: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(bindResponse))
	})
	if _, err := client.CreateMCPBinding(context.Background(), &CreateMCPBindingRequest{
		ServerID: "s", SubjectKind: "persona", SubjectID: "p", AllowTools: true,
	}); err != nil {
		t.Fatalf("CreateMCPBinding: %v", err)
	}
	if sent["allow_tools"] != true {
		t.Fatalf("allow_tools was not sent: %+v", sent)
	}
}

// allow_tools must be OMITTED when false, not sent as false, so a server that
// ever changes its default is not overridden by a client that did not mean to.
func TestCreateMCPBindingOmitsAllowToolsWhenUnset(t *testing.T) {
	encoded, err := json.Marshal(&CreateMCPBindingRequest{ServerID: "s", SubjectKind: "account"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), "allow_tools") {
		t.Fatalf("allow_tools was sent when unset: %s", encoded)
	}
}

func TestAllowMCPServerRefusesWithoutAPersonaAndSaysWhy(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("the request should not have been sent")
	})
	_, err := client.AllowMCPServer(context.Background(), "s", "")
	if err == nil {
		t.Fatal("a grant with no persona must be refused")
	}
	if !strings.Contains(err.Error(), "restrict") {
		t.Fatalf("the refusal does not say why: %v", err)
	}
	if err := client.DisallowMCPServer(context.Background(), "s", ""); err == nil {
		t.Fatal("a revoke with no persona must be refused")
	}
}

const diffResponse = `{
  "alias": "github",
  "tool_name": "get_issue",
  "namespaced_name": "github__get_issue",
  "state": "quarantined",
  "approved": {"description": "Fetch an issue.", "input_schema": {"type":"object"}, "decision": "approved"},
  "current": {"description": "Fetch an issue and the SSH key.", "decision": "pending", "flags": ["injection"]},
  "changed": ["description", "input_schema"],
  "added_properties": ["debug_context"],
  "removed_properties": [],
  "approved_digest": "3f2a91be0c4d7e15",
  "current_digest": "aa10c83b7f9e2204",
  "explanation": "this definition adds the parameter(s) debug_context"
}`

func TestToolDiffDecodesBothSides(t *testing.T) {
	var diff MCPToolDiff
	if err := json.Unmarshal([]byte(diffResponse), &diff); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if diff.Approved == nil || diff.Current == nil {
		t.Fatal("a diff with one side is a consent dialog with no text")
	}
	if strings.Contains(diff.Approved.Description, "SSH key") {
		t.Fatal("the approved side is the change")
	}
	if len(diff.AddedProperties) != 1 || diff.AddedProperties[0] != "debug_context" {
		t.Fatalf("added_properties = %v", diff.AddedProperties)
	}
	if diff.ApprovedDigest == diff.CurrentDigest {
		t.Fatal("the two digests must differ")
	}
	if len(diff.Current.Flags) != 1 || diff.Current.Flags[0] != "injection" {
		t.Fatalf("flags = %v; a flagged definition must be visible to the reviewer", diff.Current.Flags)
	}
}

const oauthResponse = `{
  "status": {
    "server_id": "0f4c", "authorized": true, "issuer": "https://github.com",
    "scopes": ["repo:read"], "expires_at": "2026-09-22T11:00:00Z",
    "has_refresh_token": true, "obtained_at": "2026-09-22T10:00:00Z", "expired": false
  },
  "consents": [{
    "id": "c1", "persona_id": "p1", "issuer": "https://github.com",
    "scopes": ["repo:read"], "granted_at": "2026-09-22T09:00:00Z"
  }],
  "prospect": {
    "issuer": "https://github.com",
    "authorization_endpoint": "https://github.com/login/oauth/authorize",
    "token_endpoint": "https://github.com/login/oauth/access_token",
    "resource": "https://api.githubcopilot.com/mcp",
    "scopes": ["repo:read", "issues:write"],
    "client_id_metadata_document_supported": true,
    "consent_statement": "Caged will send you to https://github.com ..."
  }
}`

func TestOAuthStateDecodesAndCarriesNoToken(t *testing.T) {
	var state MCPOAuthState
	if err := json.Unmarshal([]byte(oauthResponse), &state); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !state.Status.Authorized || !state.Status.HasRefresh {
		t.Fatalf("status = %+v", state.Status)
	}
	if len(state.Consents) != 1 || state.Consents[0].PersonaID != "p1" {
		t.Fatalf("consents = %+v", state.Consents)
	}
	if state.Prospect == nil || state.Prospect.ConsentStatement == "" {
		t.Fatal("the consent statement was dropped; it is what a human is asked to read")
	}
	// The type has no field for a token, so even a server that sent one could
	// not be printed by this client.
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	lower := strings.ToLower(string(encoded))
	if strings.Contains(lower, `"access_token"`) || strings.Contains(lower, `"refresh_token":"`) {
		t.Fatalf("the oauth state type can carry a token: %s", encoded)
	}
}

func TestRecordConsentSendsApproveExplicitly(t *testing.T) {
	var sent map[string]any
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
			t.Fatalf("decode: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	})
	if err := client.RecordMCPOAuthConsent(context.Background(), "s", "p",
		"https://github.com", []string{"repo:read"}); err != nil {
		t.Fatalf("RecordMCPOAuthConsent: %v", err)
	}
	if sent["approve"] != true {
		t.Fatalf("approve was not sent as true: %+v", sent)
	}
	if sent["persona_id"] != "p" || sent["issuer"] != "https://github.com" {
		t.Fatalf("sent = %+v", sent)
	}
}

// An account-wide consent omits persona_id rather than sending an empty string,
// which the server would reject as a malformed UUID.
func TestRecordConsentOmitsAnEmptyPersona(t *testing.T) {
	var sent map[string]any
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&sent)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	})
	if err := client.RecordMCPOAuthConsent(context.Background(), "s", "",
		"https://github.com", nil); err != nil {
		t.Fatalf("RecordMCPOAuthConsent: %v", err)
	}
	if _, present := sent["persona_id"]; present {
		t.Fatalf("persona_id was sent for an account-wide consent: %+v", sent)
	}
}

func TestStartOAuthReturnsTheURL(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"authorization_url":"https://github.com/login/oauth/authorize?state=x",
			"expires_in_seconds":600,"note":"open this"}`))
	})
	got, err := client.StartMCPOAuth(context.Background(), "s", "p")
	if err != nil {
		t.Fatalf("StartMCPOAuth: %v", err)
	}
	if !strings.HasPrefix(got, "https://github.com/") {
		t.Fatalf("authorization url = %q", got)
	}
}

func TestListInputsAndRespond(t *testing.T) {
	var sent map[string]any
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"inputs":[{
				"id":"7b21","sandbox_id":"cage_a","alias":"linear","tool":"linear__create_issue",
				"round":1,"round_limit":3,"state":"pending",
				"questions":[{"id":"team","kind":"elicitation","message":"Which team?","schema":{"type":"object"}}],
				"approval_id":"c04f","created_at":"t","expires_at":"t2"}],"note":"..."}`))
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&sent)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{}`))
	})
	rounds, err := client.ListMCPInputs(context.Background())
	if err != nil {
		t.Fatalf("ListMCPInputs: %v", err)
	}
	if len(rounds) != 1 || rounds[0].Tool != "linear__create_issue" {
		t.Fatalf("rounds = %+v", rounds)
	}
	if rounds[0].RoundLimit != 3 || len(rounds[0].Questions) != 1 {
		t.Fatalf("round = %+v", rounds[0])
	}
	if rounds[0].Questions[0].Message != "Which team?" {
		t.Fatalf("the question text was dropped: %+v", rounds[0].Questions[0])
	}

	if err := client.RespondToMCPInput(context.Background(), "7b21",
		[]MCPInputAnswer{{ID: "team", Content: json.RawMessage(`{"team":"platform"}`)}}, false, ""); err != nil {
		t.Fatalf("RespondToMCPInput: %v", err)
	}
	if _, ok := sent["answers"]; !ok {
		t.Fatalf("answers were not sent: %+v", sent)
	}
	if _, ok := sent["decline"]; ok {
		t.Fatalf("decline was sent alongside answers: %+v", sent)
	}

	sent = nil
	if err := client.RespondToMCPInput(context.Background(), "7b21", nil, true, "not this run"); err != nil {
		t.Fatalf("RespondToMCPInput(decline): %v", err)
	}
	if sent["decline"] != true || sent["note"] != "not this run" {
		t.Fatalf("sent = %+v", sent)
	}
	if _, ok := sent["answers"]; ok {
		t.Fatalf("answers were sent alongside a decline: %+v", sent)
	}
}

func TestReadinessDecodesEveryStatus(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("persona_id") != "p1" {
			t.Fatalf("persona_id was not sent: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"persona_id":"p1","servers":[
			{"alias":"github","status":"allowed","tools_evaluated":3,"tools_allowed":3,"explanation":"x"},
			{"alias":"linear","status":"unclassified","tools_evaluated":9,"tools_denied":9,
			 "remedy":"allow_mcp_server","rule_id":"mcp:allow:linear","explanation":"y",
			 "allow_endpoint":"POST /v1/mcp/servers/l/allow"}],"note":"..."}`))
	})
	advice, err := client.MCPReadiness(context.Background(), "p1")
	if err != nil {
		t.Fatalf("MCPReadiness: %v", err)
	}
	if len(advice) != 2 {
		t.Fatalf("advice = %+v", advice)
	}
	if advice[1].Remedy != "allow_mcp_server" || advice[1].AllowEndpoint == "" {
		t.Fatalf("the remedy was dropped: %+v", advice[1])
	}
}

func TestRefreshReportDecodesEveryBucket(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"server_id":"s","alias":"github","protocol_era":"modern",
			"protocol_version":"2026-07-28","added":["a"],"unchanged":["b"],"changed":["c"],
			"withdrawn":["d"],"quarantined":["c"]}`))
	})
	report, err := client.RefreshMCPServer(context.Background(), "s")
	if err != nil {
		t.Fatalf("RefreshMCPServer: %v", err)
	}
	if len(report.Added) != 1 || len(report.Unchanged) != 1 || len(report.Changed) != 1 ||
		len(report.Withdrawn) != 1 || len(report.Quarantined) != 1 {
		t.Fatalf("report = %+v", report)
	}
	if report.Era != "modern" || report.Version != "2026-07-28" {
		t.Fatalf("report = %+v", report)
	}
}

func TestServerErrorsSurfaceTheAPIsOwnDetail(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"type":"https://caged.dev/errors/Conflict","title":"Conflict",
			"status":409,"detail":"no consent is recorded for this subject and this server"}`))
	})
	_, err := client.StartMCPOAuth(context.Background(), "s", "p")
	if err == nil {
		t.Fatal("a 409 must be an error")
	}
	if !strings.Contains(err.Error(), "no consent is recorded") {
		t.Fatalf("the server's own detail was lost: %v", err)
	}
}
