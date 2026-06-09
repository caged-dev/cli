package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/caged-dev/cli/internal/cagefile"
)

func cmdUp(args []string) error {
	fs := flag.NewFlagSet("up", flag.ExitOnError)
	template := fs.String("template", "", "Override template from .caged.yaml")
	cpus := fs.Int("cpus", 0, "Override CPU count")
	memory := fs.Int("memory", 0, "Override memory in MB")
	disk := fs.Int("disk", 0, "Override disk in GB")
	network := fs.String("network", "", "Override network mode")
	allowlist := fs.String("allowlist", "", "Override allowlist (comma-separated)")
	repo := fs.String("repo", "", "Override repo URL")
	budgetFlag := fs.Float64("budget", 0, "Override budget in USD")
	envStr := fs.String("env", "", "Additional env vars (KEY=VAL,KEY2=VAL2)")
	configFile := fs.String("config", "", "Path to config file (default: .caged.yaml)")
	packagesStr := fs.String("packages", "", "Override packages to pre-install (comma-separated)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Find .caged.yaml.
	cfgPath := *configFile
	if cfgPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}
		cfgPath = filepath.Join(cwd, ".caged.yaml")
	}

	// Parse config file.
	var cfg *cagefile.Config
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No .caged.yaml found — using CLI flags only.")
			cfg = &cagefile.Config{}
		} else {
			return fmt.Errorf("reading %s: %w", cfgPath, err)
		}
	} else {
		fmt.Printf("Using config: %s\n", cfgPath)
		cfg, err = cagefile.Parse(data)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", cfgPath, err)
		}
	}

	// Build override from CLI flags.
	override := cagefile.Config{
		Template:    *template,
		NetworkMode: *network,
		Budget:      *budgetFlag,
		Resources: cagefile.Resources{
			CPU:    *cpus,
			Memory: *memory,
			Disk:   *disk,
		},
	}
	if *allowlist != "" {
		override.AllowedHosts = strings.Split(*allowlist, ",")
		if override.NetworkMode == "" {
			override.NetworkMode = "allowlist"
		}
	}
	if *envStr != "" {
		override.Env = parseEnvVars(*envStr)
	}
	if *packagesStr != "" {
		override.Packages = strings.Split(*packagesStr, ",")
	}

	cfg.Merge(override)

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	client, err := mustClient()
	if err != nil {
		return err
	}

	req := &createSandboxRequest{
		Template:    cfg.Template,
		CPUs:        cfg.Resources.CPU,
		MemoryMB:    cfg.Resources.Memory,
		DiskGB:      cfg.Resources.Disk,
		NetworkMode: cfg.NetworkMode,
		BudgetUSD:   cfg.Budget,
	}
	if len(cfg.AllowedHosts) > 0 {
		req.Allowlist = cfg.AllowedHosts
	}
	if len(cfg.Env) > 0 {
		req.Env = cfg.Env
	}
	if *repo != "" {
		req.Repo = *repo
	}
	if len(cfg.Packages) > 0 {
		req.Packages = cfg.Packages
	}

	fmt.Printf("Creating sandbox (template=%s", cfg.Template)
	if cfg.Resources.CPU > 0 {
		fmt.Printf(", cpus=%d", cfg.Resources.CPU)
	}
	if cfg.Resources.Memory > 0 {
		fmt.Printf(", mem=%dMB", cfg.Resources.Memory)
	}
	if cfg.Budget > 0 {
		fmt.Printf(", budget=$%.2f", cfg.Budget)
	}
	fmt.Println(")...")

	ctx := context.Background()
	sandbox, err := client.CreateSandbox(ctx, req)
	if err != nil {
		return fmt.Errorf("creating sandbox: %w", err)
	}

	printSandboxInfo(sandbox)
	fmt.Printf("\nConnect: caged connect %s\n", sandbox.ID)
	return nil
}
