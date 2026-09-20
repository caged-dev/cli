package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/caged-dev/cli/internal/cagefile"
)

// TestBuildCreateRequest_CarriesEveryConfigField is the regression guard for
// the whole "parsed and then dropped" bug class: a .caged.yaml setting that
// reaches cagefile.Config but not the create payload is a setting the user
// wrote and nobody applied. timeout, secrets and init_script were dropped
// exactly that way.
func TestBuildCreateRequest_CarriesEveryConfigField(t *testing.T) {
	t.Setenv("TEST_REPO_TOKEN", "ghp_from_env")

	const yaml = `
template: node-22
resources:
  cpu: 4
  memory: 2048
  disk: 20
timeout: 900
budget: 5.5
init_script: npm ci && npm run build
network_mode: allowlist
allowed_hosts:
  - registry.npmjs.org
secrets:
  - ANTHROPIC_API_KEY
  - OPENAI_API_KEY
packages:
  - typescript
agents:
  - claude-code
env:
  NODE_ENV: test
repo:
  url: https://github.com/org/repo
  token_env: TEST_REPO_TOKEN
  branch: dev
  commit: abc123
  subdirectory: services/api
`
	cfg, err := cagefile.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	req := buildCreateRequest(cfg)

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Field names are those of the server's CreateSandboxRequest; a rename
	// on either side has to break this test.
	tests := []struct {
		field string
		want  any
	}{
		{"template", "node-22"},
		{"cpus", float64(4)},
		{"memory_mb", float64(2048)},
		{"disk_gb", float64(20)},
		{"timeout", float64(900)},
		{"budget", 5.5},
		{"init_script", "npm ci && npm run build"},
		{"network_mode", "allowlist"},
		{"allowlist", []any{"registry.npmjs.org"}},
		{"secrets", []any{"ANTHROPIC_API_KEY", "OPENAI_API_KEY"}},
		{"packages", []any{"typescript"}},
		{"agents", []any{"claude-code"}},
		{"env", map[string]any{"NODE_ENV": "test"}},
		{"repo", "https://github.com/org/repo"},
		{"repo_token", "ghp_from_env"},
		{"repo_branch", "dev"},
		{"repo_commit", "abc123"},
		{"repo_subdir", "services/api"},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			got, ok := wire[tt.field]
			if !ok {
				t.Fatalf("field %q missing from create request body: %s", tt.field, body)
			}
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(tt.want)
			if !bytes.Equal(gotJSON, wantJSON) {
				t.Errorf("%s = %s, want %s", tt.field, gotJSON, wantJSON)
			}
		})
	}
}

func TestBuildCreateRequest_OmitsUnsetFields(t *testing.T) {
	cfg, err := cagefile.Parse([]byte("template: python-312\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	body, err := json.Marshal(buildCreateRequest(cfg))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, field := range []string{"timeout", "secrets", "init_script", "budget", "allowlist", "env", "repo"} {
		if _, ok := wire[field]; ok {
			t.Errorf("field %q sent despite not being configured: %s", field, body)
		}
	}
}

func TestBuildCreateRequest_RepoTokenPrefersLiteral(t *testing.T) {
	t.Setenv("TEST_REPO_TOKEN", "from_env")
	cfg := &cagefile.Config{
		Template: "node-22",
		Repo: cagefile.RepoConfig{
			URL:      "https://github.com/org/repo",
			Token:    "literal",
			TokenEnv: "TEST_REPO_TOKEN",
		},
	}
	if got := buildCreateRequest(cfg).RepoToken; got != "literal" {
		t.Errorf("RepoToken = %q, want literal to win over token_env", got)
	}
}

func TestWarnUnappliedConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  cagefile.Config
		want []string
	}{
		{"nothing unapplied", cagefile.Config{Template: "node-22"}, nil},
		{"secrets only", cagefile.Config{Secrets: []string{"A"}}, []string{"secrets"}},
		{"init script only", cagefile.Config{InitScript: "make"}, []string{"init_script"}},
		{"both", cagefile.Config{Secrets: []string{"A"}, InitScript: "make"}, []string{"secrets", "init_script"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			warnUnappliedConfig(&buf, &tt.cfg)
			out := buf.String()
			if len(tt.want) == 0 {
				if out != "" {
					t.Fatalf("unexpected warning: %q", out)
				}
				return
			}
			if !strings.HasPrefix(out, "warning: ") {
				t.Errorf("warning not prefixed: %q", out)
			}
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("warning %q does not mention %q", out, w)
				}
			}
		})
	}
}
