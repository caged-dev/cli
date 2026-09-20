package api

import (
	"encoding/json"
	"reflect"
	"testing"
)

// agentDefinitionFile is an agent registration a user hands to
// `caged a2a agent create -f`, written against the server's a2a.Skill.
// Everything in it has to survive the client's decode / re-encode.
const agentDefinitionFile = `{
  "name": "reviewer",
  "description": "reviews diffs",
  "template": "node-22",
  "public": true,
  "allowed_orgs": ["acme"],
  "max_cost_per_task": 0.5,
  "rate_limit_rpm": 60,
  "skills": [
    {
      "id": "review",
      "name": "Review a diff",
      "description": "comments on a patch",
      "tags": ["code"],
      "input_schema": {"type": "object"},
      "output_schema": {"type": "string"},
      "examples": [{"input": {"diff": "x"}, "output": "looks fine"}],
      "metadata": {"tier": "pro"}
    }
  ]
}`

func TestCreateA2AAgentRequest_RoundTripsEveryField(t *testing.T) {
	var req CreateA2AAgentRequest
	if err := json.Unmarshal([]byte(agentDefinitionFile), &req); err != nil {
		t.Fatalf("decode: %v", err)
	}
	reencoded, err := json.Marshal(&req)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var want, got map[string]any
	if err := json.Unmarshal([]byte(agentDefinitionFile), &want); err != nil {
		t.Fatalf("decode want: %v", err)
	}
	if err := json.Unmarshal(reencoded, &got); err != nil {
		t.Fatalf("decode got: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("agent registration changed passing through the client\n got: %s\nwant: %s", reencoded, agentDefinitionFile)
	}
}

// taskResponseBody is a task as an A2A server serves it: the field set of
// caged-api's a2a.Task. `caged a2a task get` prints the decoded task as
// JSON, so anything the client does not declare vanishes from the user's
// view of the task — artifacts, budget and the message history did.
const taskResponseBody = `{
  "id": "7c6e5b6e-0000-4000-8000-000000000001",
  "session_id": "sess_1",
  "parent_task_id": "7c6e5b6e-0000-4000-8000-000000000002",
  "from_agent_url": "https://caged.dev/a2a",
  "to_agent_url": "https://agent.example.com",
  "skill_id": "review",
  "status": "completed",
  "status_message": "done",
  "progress": {"percentage": 100, "current_step": "report", "total_steps": 3, "step_number": 3, "message": "finished"},
  "messages": [
    {
      "id": "7c6e5b6e-0000-4000-8000-000000000003",
      "task_id": "7c6e5b6e-0000-4000-8000-000000000001",
      "role": "agent",
      "parts": [{"type": "code", "text": "x := 1", "language": "go", "mime_type": "text/x-go", "name": "main.go", "uri": "s3://b/k", "size": 7, "checksum": "abc", "content": "eA==", "encoding": "base64", "data": {"k": "v"}}],
      "metadata": {"turn": "1"},
      "created_at": "2026-09-20T10:00:01Z"
    }
  ],
  "input": {"diff": "x"},
  "output": "looks fine",
  "artifacts": [{"id": "7c6e5b6e-0000-4000-8000-000000000004", "task_id": "7c6e5b6e-0000-4000-8000-000000000001", "name": "report.md", "mime_type": "text/markdown", "size": 120, "uri": "s3://b/report.md", "checksum": "def", "created_at": "2026-09-20T10:00:02Z"}],
  "priority": 5,
  "deadline": "2026-09-20T11:00:00Z",
  "budget": {"max_cost_usd": 0.5, "max_duration": 600000000000, "max_tokens": 10000, "max_iterations": 4},
  "metadata": {"tier": "pro"},
  "tags": ["review"],
  "created_at": "2026-09-20T10:00:00Z",
  "updated_at": "2026-09-20T10:00:03Z",
  "started_at": "2026-09-20T10:00:01Z",
  "completed_at": "2026-09-20T10:00:03Z",
  "auth_context": {"agent_identity": "tok", "session_id": "sess_1", "account_id": "acct_1", "scopes": ["a2a:task"], "claims": {"plan": "pro"}}
}`

func TestA2ATask_RoundTripsEveryField(t *testing.T) {
	var task A2ATask
	if err := json.Unmarshal([]byte(taskResponseBody), &task); err != nil {
		t.Fatalf("decode: %v", err)
	}
	reencoded, err := json.Marshal(&task)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var want, got map[string]any
	if err := json.Unmarshal([]byte(taskResponseBody), &want); err != nil {
		t.Fatalf("decode want: %v", err)
	}
	if err := json.Unmarshal(reencoded, &got); err != nil {
		t.Fatalf("decode got: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("task changed passing through the client\n got: %s\nwant: %s", reencoded, taskResponseBody)
	}
}

// agentCardBody is an Agent Card as the discovery endpoint serves it. The
// Caged half of the card — the agent's trust score among it — lives under
// extensions, which the client used not to declare at all.
const agentCardBody = `{
  "name": "reviewer",
  "description": "reviews diffs",
  "url": "https://agent.example.com/a2a",
  "version": "1.0",
  "capabilities": ["tasks"],
  "input_modes": ["text"],
  "output_modes": ["text"],
  "streaming_mode": "sse",
  "authentication": {"type": "oauth2", "schemes": ["bearer"], "token_url": "https://agent.example.com/token", "scopes": ["a2a"], "instructions": "use a bearer token"},
  "skills": [{"id": "review", "name": "Review", "tags": ["code"]}],
  "provider": {"name": "Acme", "organization": "acme", "url": "https://acme.test", "contact_email": "a2a@acme.test"},
  "signature": "jws",
  "public_key_id": "key-1",
  "verified": true,
  "expires_at": "2026-12-01T00:00:00Z",
  "extensions": {"caged_agent_id": "agt_1", "pipeline_id": "pl_1", "template": "node-22", "trust_score": 88, "max_budget": 2.5, "network_mode": "allowlist"}
}`

func TestA2AAgentCard_RoundTripsEveryField(t *testing.T) {
	var card A2AAgentCard
	if err := json.Unmarshal([]byte(agentCardBody), &card); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if card.Extensions == nil || card.Extensions.TrustScore != 88 {
		t.Fatalf("extensions lost: %+v", card.Extensions)
	}
	reencoded, err := json.Marshal(&card)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var want, got map[string]any
	if err := json.Unmarshal([]byte(agentCardBody), &want); err != nil {
		t.Fatalf("decode want: %v", err)
	}
	if err := json.Unmarshal(reencoded, &got); err != nil {
		t.Fatalf("decode got: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("agent card changed passing through the client\n got: %s\nwant: %s", reencoded, agentCardBody)
	}
}
