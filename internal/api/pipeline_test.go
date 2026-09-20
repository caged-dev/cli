package api

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// pipelineDefinitionFile is a definition a user can hand to
// `caged pipeline create -f`, written against the server's
// pipeline.StageDefinition. Everything in it has to survive the decode /
// re-encode the CLI performs: a stripped `config` block turns a gate stage
// into a gate with no conditions.
const pipelineDefinitionFile = `{
  "name": "ship",
  "description": "build, gate, deploy",
  "stages": [
    {
      "name": "build",
      "type": "command",
      "description": "compile",
      "command": "make build",
      "template": "node-22",
      "env": {"CI": "1"},
      "timeout": 300000000000,
      "retry": {"max_attempts": 3, "backoff": 1000000000, "max_backoff": 30000000000},
      "on_failure": "retry"
    },
    {
      "name": "gate",
      "type": "gate",
      "depends_on": ["build"],
      "condition": {"on_success": true},
      "config": {"trust_above": 70, "cost_below": 2.5, "fail_fast": true}
    }
  ],
  "defaults": {
    "template": "node-22",
    "timeout": 600000000000,
    "on_failure": "stop",
    "retry": {"max_attempts": 2, "backoff": 2000000000, "max_backoff": 60000000000}
  }
}`

func TestCreatePipelineRequest_RoundTripsEveryField(t *testing.T) {
	var req CreatePipelineRequest
	if err := json.Unmarshal([]byte(pipelineDefinitionFile), &req); err != nil {
		t.Fatalf("decoding definition: %v", err)
	}

	reencoded, err := json.Marshal(&req)
	if err != nil {
		t.Fatalf("encoding definition: %v", err)
	}

	var want, got map[string]any
	if err := json.Unmarshal([]byte(pipelineDefinitionFile), &want); err != nil {
		t.Fatalf("decoding want: %v", err)
	}
	if err := json.Unmarshal(reencoded, &got); err != nil {
		t.Fatalf("decoding got: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("pipeline definition changed passing through the client\n got: %s\nwant: %s", reencoded, pipelineDefinitionFile)
	}
}

func TestStageDefinition_Fields(t *testing.T) {
	var req CreatePipelineRequest
	if err := json.Unmarshal([]byte(pipelineDefinitionFile), &req); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(req.Stages) != 2 {
		t.Fatalf("stages = %d, want 2", len(req.Stages))
	}
	build := req.Stages[0]
	tests := []struct {
		field string
		got   any
		want  any
	}{
		{"Description", build.Description, "compile"},
		{"Env[CI]", build.Env["CI"], "1"},
		{"Timeout", build.Timeout, 5 * time.Minute},
		{"OnFailure", build.OnFailure, "retry"},
		{"Retry.MaxAttempts", build.Retry.MaxAttempts, 3},
		{"Retry.Backoff", build.Retry.Backoff, time.Second},
		{"Defaults.Timeout", req.Defaults.Timeout, 10 * time.Minute},
		{"Defaults.OnFailure", req.Defaults.OnFailure, "stop"},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %v", tt.field, tt.got, tt.want)
			}
		})
	}
	if len(req.Stages[1].Config) == 0 {
		t.Error("gate stage config dropped")
	}
}

// runResponseBody is a run as the API serves it: caged-api's
// pipeline.Run, whose end-of-run field is completed_at. The client used to
// read ended_at, which no server has ever sent, so `caged pipeline runs`
// printed "-" for every finished run.
const runResponseBody = `{
  "id": "7c6e5b6e-0000-4000-8000-000000000001",
  "pipeline_id": "7c6e5b6e-0000-4000-8000-000000000002",
  "pipeline_name": "ship",
  "account_id": "7c6e5b6e-0000-4000-8000-000000000003",
  "status": "failed",
  "trigger": "cli",
  "input": {"repo": "https://github.com/org/repo", "branch": "main", "commit": "abc123", "env": {"CI": "1"}, "variables": {"tier": "pro"}},
  "started_at": "2026-09-20T10:00:00Z",
  "completed_at": "2026-09-20T10:04:12Z",
  "duration_ms": 252000,
  "error_message": "stage build failed",
  "created_at": "2026-09-20T09:59:58Z",
  "updated_at": "2026-09-20T10:04:12Z"
}`

func TestRunDecode_ServerBody(t *testing.T) {
	var r Run
	if err := json.Unmarshal([]byte(runResponseBody), &r); err != nil {
		t.Fatalf("decode: %v", err)
	}
	tests := []struct {
		field string
		got   any
		want  any
	}{
		{"PipelineName", r.PipelineName, "ship"},
		{"Status", r.Status, "failed"},
		{"StartedAt", r.StartedAt, "2026-09-20T10:00:00Z"},
		{"CompletedAt", r.CompletedAt, "2026-09-20T10:04:12Z"},
		{"DurationMS", r.DurationMS, int64(252000)},
		{"ErrorMessage", r.ErrorMessage, "stage build failed"},
		{"UpdatedAt", r.UpdatedAt, "2026-09-20T10:04:12Z"},
		{"Input.Commit", r.Input.Commit, "abc123"},
		{"Input.Variables[tier]", r.Input.Variables["tier"], "pro"},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %v", tt.field, tt.got, tt.want)
			}
		})
	}
}

// TestRunHasNoPhantomFields is the Run-shaped version of the sandbox guard.
func TestRunHasNoPhantomFields(t *testing.T) {
	served := map[string]bool{
		"id": true, "pipeline_id": true, "pipeline_name": true,
		"account_id": true, "status": true, "trigger": true, "input": true,
		"started_at": true, "completed_at": true, "duration_ms": true,
		"error_message": true, "created_at": true, "updated_at": true,
	}
	full := Run{ID: "x", PipelineID: "x", PipelineName: "x", AccountID: "x",
		Status: "x", Trigger: "x", Input: RunInput{Repo: "x"}, StartedAt: "x",
		CompletedAt: "x", DurationMS: 1, ErrorMessage: "x", CreatedAt: "x",
		UpdatedAt: "x"}
	body, err := json.Marshal(full)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for field := range wire {
		if !served[field] {
			t.Errorf("Run declares %q, which the API never sends", field)
		}
	}
}
