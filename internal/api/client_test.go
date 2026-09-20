package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// sandboxResponseBody is a POST /v1/sandboxes response as the API serves
// it: the exact field set of caged-api's api.SandboxResponse, with the
// fields toSandboxResponse populates. Note `cost` (always sent, including
// zero) and the absence of `spent`, which no version of the API has sent.
const sandboxResponseBody = `{
  "id": "cage_abc123",
  "status": "running",
  "template": "node-22",
  "ip": "172.30.0.42",
  "cpus": 2,
  "memory_mb": 1024,
  "disk_gb": 10,
  "network_mode": "allowlist",
  "repo_url": "https://github.com/org/repo",
  "budget": 5,
  "created_at": "2026-09-20T10:00:00Z",
  "started_at": "2026-09-20T10:00:04Z",
  "cost": 1.37
}`

func TestSandboxDecode_ServerBody(t *testing.T) {
	var s Sandbox
	if err := json.Unmarshal([]byte(sandboxResponseBody), &s); err != nil {
		t.Fatalf("decoding sandbox: %v", err)
	}

	tests := []struct {
		field string
		got   any
		want  any
	}{
		{"ID", s.ID, "cage_abc123"},
		{"Status", s.Status, "running"},
		{"Template", s.Template, "node-22"},
		{"IP", s.IP, "172.30.0.42"},
		{"CPUs", s.CPUs, 2},
		{"MemoryMB", s.MemoryMB, 1024},
		{"DiskGB", s.DiskGB, 10},
		{"NetworkMode", s.NetworkMode, "allowlist"},
		{"RepoURL", s.RepoURL, "https://github.com/org/repo"},
		{"Budget", s.Budget, 5.0},
		{"Cost", s.Cost, 1.37},
		{"CreatedAt", s.CreatedAt, "2026-09-20T10:00:00Z"},
		{"StartedAt", s.StartedAt, "2026-09-20T10:00:04Z"},
		{"StoppedAt", s.StoppedAt, ""},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %v", tt.field, tt.got, tt.want)
			}
		})
	}
}

// TestSandboxDecode_ZeroCostIsSent guards the distinction the API is
// explicit about: cost is sent even when it is zero, so a decoded 0 means
// "nothing spent yet" rather than "field absent".
func TestSandboxDecode_ZeroCostIsSent(t *testing.T) {
	var s Sandbox
	if err := json.Unmarshal([]byte(`{"id":"cage_new","status":"creating","cost":0}`), &s); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if s.Cost != 0 {
		t.Errorf("Cost = %v, want 0", s.Cost)
	}
}

// TestSandboxHasNoPhantomFields fails if a field is ever added to Sandbox
// that the API does not send. Any such field reads zero forever and is
// indistinguishable from data.
func TestSandboxHasNoPhantomFields(t *testing.T) {
	// The field set of caged-api's api.SandboxResponse.
	served := map[string]bool{
		"id": true, "status": true, "template": true, "ip": true,
		"cpus": true, "memory_mb": true, "disk_gb": true,
		"network_mode": true, "repo_url": true, "budget": true,
		"created_at": true, "started_at": true, "stopped_at": true,
		"cost": true,
		// Declared by SandboxResponse but never populated by any server
		// path, so the client deliberately does not declare them:
		// "init_script", "timeout", "config".
	}
	// Every field non-zero, so omitempty hides none of them.
	full := Sandbox{ID: "x", Status: "x", Template: "x", IP: "x", CPUs: 1,
		MemoryMB: 1, DiskGB: 1, NetworkMode: "x", RepoURL: "x",
		CreatedAt: "x", StartedAt: "x", StoppedAt: "x", Budget: 1, Cost: 1}
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
			t.Errorf("Sandbox declares %q, which the API never sends", field)
		}
	}
}

func TestCreateSandbox_SendsRequestBody(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decoding request: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(sandboxResponseBody))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key")
	sb, err := c.CreateSandbox(context.Background(), &CreateSandboxRequest{
		Template:   "node-22",
		Timeout:    900,
		Secrets:    []string{"ANTHROPIC_API_KEY"},
		InitScript: "npm ci",
	})
	if err != nil {
		t.Fatalf("CreateSandbox: %v", err)
	}
	for _, field := range []string{"timeout", "secrets", "init_script"} {
		if _, ok := got[field]; !ok {
			t.Errorf("request body missing %q: %v", field, got)
		}
	}
	if sb.Cost != 1.37 {
		t.Errorf("Cost = %v, want 1.37", sb.Cost)
	}
}

func TestDo_APIErrorIsStructured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		// RFC 7807 ProblemDetail, as the API's writeError emits it.
		_, _ = w.Write([]byte(`{"type":"https://caged.dev/errors/Bad Request","title":"invalid request","status":400,"detail":"timeout cannot be negative"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key")
	_, err := c.GetSandbox(context.Background(), "cage_abc")
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error is %T, want *APIError", err)
	}
	if apiErr.Status != http.StatusBadRequest {
		t.Errorf("Status = %d, want 400", apiErr.Status)
	}
	if apiErr.Error() != "invalid request: timeout cannot be negative" {
		t.Errorf("Error() = %q", apiErr.Error())
	}
}
