package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/caged-dev/cli/internal/cagefile"
)

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	template := fs.String("template", "node", "Sandbox template (node, python, or full: node-22, python-312, minimal)")
	// Ranges come from cagefile.DefaultLimits, which mirrors the API's
	// own bounds — see internal/cagefile/limits.go.
	cpus := fs.Int("cpus", 2, fmt.Sprintf("Number of vCPUs (1-%d)", cagefile.MaxCPU))
	memory := fs.Int("memory", 512, fmt.Sprintf("Memory in MB (1-%d)", cagefile.MaxMemoryMB))
	disk := fs.Int("disk", 5, fmt.Sprintf("Disk in GB (1-%d)", cagefile.MaxDiskGB))
	network := fs.String("network", "full", "Network mode: full, none, allowlist")
	allowlist := fs.String("allowlist", "", "Comma-separated host allowlist")
	repo := fs.String("repo", "", "Git repository to clone")
	envStr := fs.String("env", "", "Environment variables (KEY=VAL,KEY2=VAL2)")
	// Not a cap: the platform records a sandbox budget and reports cost
	// against it, but nothing stops a sandbox that exceeds it.
	budgetFlag := fs.Float64("budget", 0, "Budget in USD to record for this sandbox (reported against, not enforced)")
	timeoutFlag := fs.Int("timeout", 0, "Idle timeout in seconds before the sandbox sleeps (0 = server default)")
	packagesStr := fs.String("packages", "", "Comma-separated packages to pre-install (e.g., @anthropic-ai/claude-code,typescript)")
	agentsStr := fs.String("agents", "", "Comma-separated AI agents to install (e.g., claude-code,aider,codex)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	client, err := mustClient()
	if err != nil {
		return err
	}

	req := &createSandboxRequest{
		Template:    *template,
		CPUs:        *cpus,
		MemoryMB:    *memory,
		DiskGB:      *disk,
		NetworkMode: *network,
	}

	if *allowlist != "" {
		req.Allowlist = strings.Split(*allowlist, ",")
	}
	if *envStr != "" {
		req.Env = parseEnvVars(*envStr)
	}
	if *repo != "" {
		req.Repo = *repo
	}
	if *budgetFlag > 0 {
		req.Budget = *budgetFlag
	}
	if *timeoutFlag > 0 {
		req.Timeout = *timeoutFlag
	}
	if *packagesStr != "" {
		req.Packages = strings.Split(*packagesStr, ",")
	}
	if *agentsStr != "" {
		req.Agents = strings.Split(*agentsStr, ",")
	}

	fmt.Printf("Creating sandbox (template=%s, cpus=%d, mem=%dMB)...\n", req.Template, req.CPUs, req.MemoryMB)

	ctx := context.Background()
	sandbox, err := client.CreateSandbox(ctx, req)
	if err != nil {
		return fmt.Errorf("creating sandbox: %w", err)
	}

	printSandboxInfo(sandbox)
	fmt.Printf("\nConnect: caged connect %s\n", sandbox.ID)
	return nil
}

func parseEnvVars(s string) map[string]string {
	env := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			env[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return env
}
