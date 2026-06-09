package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
)

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	template := fs.String("template", "node-20", "Sandbox template (e.g., node-20, python-3.12)")
	cpus := fs.Int("cpus", 2, "Number of vCPUs")
	memory := fs.Int("memory", 512, "Memory in MB")
	disk := fs.Int("disk", 5, "Disk in GB")
	network := fs.String("network", "full", "Network mode: full, none, allowlist")
	allowlist := fs.String("allowlist", "", "Comma-separated host allowlist")
	repo := fs.String("repo", "", "Git repository to clone")
	envStr := fs.String("env", "", "Environment variables (KEY=VAL,KEY2=VAL2)")
	budgetFlag := fs.Float64("budget", 0, "Maximum spend in USD")

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
		req.BudgetUSD = *budgetFlag
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
