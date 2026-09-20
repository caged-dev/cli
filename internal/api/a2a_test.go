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
