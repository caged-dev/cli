package cagefile

import (
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
