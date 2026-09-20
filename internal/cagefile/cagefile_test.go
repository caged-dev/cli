package cagefile

import (
	"strings"
	"testing"
)

func TestParse_EnvExpansion(t *testing.T) {
	t.Setenv("TEST_CAGED_SECRET", "sk-test-123")

	yaml := `
template: python
env:
  PLAIN: "value"
  SECRET: ${TEST_CAGED_SECRET}
  MISSING: ${TEST_CAGED_UNSET_VAR}
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if cfg.Env["PLAIN"] != "value" {
		t.Errorf("PLAIN = %q, want %q", cfg.Env["PLAIN"], "value")
	}
	if cfg.Env["SECRET"] != "sk-test-123" {
		t.Errorf("SECRET = %q, want expanded value", cfg.Env["SECRET"])
	}
	if cfg.Env["MISSING"] != "" {
		t.Errorf("MISSING = %q, want empty for unset var", cfg.Env["MISSING"])
	}
}

func TestParse_ResourceAliases(t *testing.T) {
	yaml := `
template: python
cpus: 2
memory: 1024
disk: 10
network: full
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Resources.CPU != 2 || cfg.Resources.Memory != 1024 || cfg.Resources.Disk != 10 {
		t.Errorf("aliases not merged: %+v", cfg.Resources)
	}
	if cfg.NetworkMode != "full" {
		t.Errorf("network alias not merged: %q", cfg.NetworkMode)
	}
}

func TestParse_DroppedFields(t *testing.T) {
	yaml := `
template: node
timeout: 900
init_script: npm ci
secrets:
  - ANTHROPIC_API_KEY
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Timeout != 900 {
		t.Errorf("Timeout = %d, want 900", cfg.Timeout)
	}
	if cfg.InitScript != "npm ci" {
		t.Errorf("InitScript = %q", cfg.InitScript)
	}
	if len(cfg.Secrets) != 1 || cfg.Secrets[0] != "ANTHROPIC_API_KEY" {
		t.Errorf("Secrets = %v", cfg.Secrets)
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name     string
		base     Config
		override Config
		check    func(*testing.T, *Config)
	}{
		{
			name:     "timeout override",
			base:     Config{Timeout: 300},
			override: Config{Timeout: 900},
			check: func(t *testing.T, c *Config) {
				if c.Timeout != 900 {
					t.Errorf("Timeout = %d, want 900", c.Timeout)
				}
			},
		},
		{
			name:     "zero timeout keeps yaml value",
			base:     Config{Timeout: 300},
			override: Config{},
			check: func(t *testing.T, c *Config) {
				if c.Timeout != 300 {
					t.Errorf("Timeout = %d, want 300", c.Timeout)
				}
			},
		},
		{
			name:     "secrets override",
			base:     Config{Secrets: []string{"A"}},
			override: Config{Secrets: []string{"B", "C"}},
			check: func(t *testing.T, c *Config) {
				if len(c.Secrets) != 2 || c.Secrets[0] != "B" {
					t.Errorf("Secrets = %v, want [B C]", c.Secrets)
				}
			},
		},
		{
			name:     "empty secrets keeps yaml value",
			base:     Config{Secrets: []string{"A"}},
			override: Config{},
			check: func(t *testing.T, c *Config) {
				if len(c.Secrets) != 1 || c.Secrets[0] != "A" {
					t.Errorf("Secrets = %v, want [A]", c.Secrets)
				}
			},
		},
		{
			name: "flags override everything they set",
			base: Config{Template: "node", Resources: Resources{CPU: 1, Memory: 256, Disk: 5},
				Timeout: 300, Budget: 1, InitScript: "make", NetworkMode: "full",
				AllowedHosts: []string{"a"}, Env: map[string]string{"A": "1"},
				Packages: []string{"p"}, Agents: []string{"aider"},
				Repo: RepoConfig{URL: "u", Token: "t", TokenEnv: "te", Branch: "b", Commit: "c", Subdirectory: "s"}},
			override: Config{Template: "python", Resources: Resources{CPU: 4, Memory: 2048, Disk: 20},
				Timeout: 900, Budget: 9, InitScript: "npm ci", NetworkMode: "none",
				AllowedHosts: []string{"b"}, Env: map[string]string{"B": "2"},
				Packages: []string{"q"}, Agents: []string{"claude-code"},
				Repo: RepoConfig{URL: "u2", Token: "t2", TokenEnv: "te2", Branch: "b2", Commit: "c2", Subdirectory: "s2"}},
			check: func(t *testing.T, c *Config) {
				if c.Template != "python" || c.Resources.CPU != 4 || c.Resources.Memory != 2048 ||
					c.Resources.Disk != 20 || c.Timeout != 900 || c.Budget != 9 ||
					c.InitScript != "npm ci" || c.NetworkMode != "none" ||
					len(c.AllowedHosts) != 1 || c.AllowedHosts[0] != "b" ||
					c.Packages[0] != "q" || c.Agents[0] != "claude-code" ||
					c.Repo.URL != "u2" || c.Repo.Token != "t2" || c.Repo.TokenEnv != "te2" ||
					c.Repo.Branch != "b2" || c.Repo.Commit != "c2" || c.Repo.Subdirectory != "s2" {
					t.Errorf("override not fully applied: %+v", c)
				}
				// Env merges rather than replacing.
				if c.Env["A"] != "1" || c.Env["B"] != "2" {
					t.Errorf("Env = %v, want both keys", c.Env)
				}
			},
		},
		{
			name:     "init script override",
			base:     Config{InitScript: "make"},
			override: Config{InitScript: "npm ci"},
			check: func(t *testing.T, c *Config) {
				if c.InitScript != "npm ci" {
					t.Errorf("InitScript = %q", c.InitScript)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.base
			cfg.Merge(tt.override)
			tt.check(t, &cfg)
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{"valid", Config{Template: "node", Timeout: 900}, ""},
		{"no template", Config{}, "template is required"},
		{"negative timeout", Config{Template: "node", Timeout: -1}, "timeout must be non-negative"},
		{"negative budget", Config{Template: "node", Budget: -1}, "budget must be non-negative"},
		{"allowlist without hosts", Config{Template: "node", NetworkMode: "allowlist"}, "allowed_hosts required"},
		{"bad network mode", Config{Template: "node", NetworkMode: "bridge"}, "network_mode must be"},
		{"cpu out of range", Config{Template: "node", Resources: Resources{CPU: 99}}, "resources.cpu"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}
