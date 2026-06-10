// Package cagefile handles .caged.yaml parsing and validation.
package cagefile

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents a parsed .caged.yaml file.
type Config struct {
	Template     string            `yaml:"template" json:"template"`
	Resources    Resources         `yaml:"resources" json:"resources"`
	Timeout      int               `yaml:"timeout" json:"timeout,omitempty"`
	Budget       float64           `yaml:"budget" json:"budget,omitempty"`
	InitScript   string            `yaml:"init_script" json:"init_script,omitempty"`
	Env          map[string]string `yaml:"env" json:"env,omitempty"`
	NetworkMode  string            `yaml:"network_mode" json:"network_mode,omitempty"`
	AllowedHosts []string          `yaml:"allowed_hosts" json:"allowed_hosts,omitempty"`
	Secrets      []string          `yaml:"secrets" json:"secrets,omitempty"`
	Packages     []string          `yaml:"packages" json:"packages,omitempty"` // Pre-install packages
	Agents       []string          `yaml:"agents" json:"agents,omitempty"`     // AI agents to install
}

// Resources defines sandbox resource limits.
type Resources struct {
	CPU    int `yaml:"cpu" json:"cpu"`
	Memory int `yaml:"memory" json:"memory"` // MB
	Disk   int `yaml:"disk" json:"disk"`     // GB
}

// Parse parses .caged.yaml content from bytes.
func Parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing yaml: %w", err)
	}
	return &cfg, nil
}

// Validate checks the config for errors.
func (c *Config) Validate() error {
	var errs []string

	if c.Template == "" {
		errs = append(errs, "template is required (e.g., 'node-20', 'python-3.12')")
	}
	if c.Resources.CPU < 0 || c.Resources.CPU > 16 {
		errs = append(errs, fmt.Sprintf("resources.cpu must be 1-16, got %d", c.Resources.CPU))
	}
	if c.Resources.Memory < 0 || c.Resources.Memory > 32768 {
		errs = append(errs, fmt.Sprintf("resources.memory must be 128-32768 (MB), got %d", c.Resources.Memory))
	}
	if c.Resources.Disk < 0 || c.Resources.Disk > 100 {
		errs = append(errs, fmt.Sprintf("resources.disk must be 1-100 (GB), got %d", c.Resources.Disk))
	}
	if c.Budget < 0 {
		errs = append(errs, "budget must be non-negative")
	}

	validModes := map[string]bool{"": true, "full": true, "none": true, "allowlist": true}
	if !validModes[c.NetworkMode] {
		errs = append(errs, fmt.Sprintf("network_mode must be 'full', 'none', or 'allowlist', got %q", c.NetworkMode))
	}
	if c.NetworkMode == "allowlist" && len(c.AllowedHosts) == 0 {
		errs = append(errs, "allowed_hosts required when network_mode is 'allowlist'")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// Merge applies CLI flag overrides on top of the yaml config.
func (c *Config) Merge(override Config) {
	if override.Template != "" {
		c.Template = override.Template
	}
	if override.Resources.CPU > 0 {
		c.Resources.CPU = override.Resources.CPU
	}
	if override.Resources.Memory > 0 {
		c.Resources.Memory = override.Resources.Memory
	}
	if override.Resources.Disk > 0 {
		c.Resources.Disk = override.Resources.Disk
	}
	if override.Timeout > 0 {
		c.Timeout = override.Timeout
	}
	if override.Budget > 0 {
		c.Budget = override.Budget
	}
	if override.InitScript != "" {
		c.InitScript = override.InitScript
	}
	if override.NetworkMode != "" {
		c.NetworkMode = override.NetworkMode
	}
	if len(override.AllowedHosts) > 0 {
		c.AllowedHosts = override.AllowedHosts
	}
	if len(override.Env) > 0 {
		if c.Env == nil {
			c.Env = make(map[string]string)
		}
		for k, v := range override.Env {
			c.Env[k] = v
		}
	}
	if len(override.Packages) > 0 {
		c.Packages = override.Packages
	}
}
