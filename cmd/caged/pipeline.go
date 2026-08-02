package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/caged-dev/cli/internal/api"
)

func cmdPipeline(args []string) error {
	if len(args) == 0 {
		printPipelineUsage()
		return nil
	}

	switch args[0] {
	case "create":
		return cmdPipelineCreate(args[1:])
	case "list", "ls":
		return cmdPipelineList(args[1:])
	case "get":
		return cmdPipelineGet(args[1:])
	case "delete", "rm":
		return cmdPipelineDelete(args[1:])
	case "run":
		return cmdPipelineRun(args[1:])
	case "runs":
		return cmdPipelineRuns(args[1:])
	case "cancel":
		return cmdPipelineCancel(args[1:])
	case "help", "--help", "-h":
		printPipelineUsage()
		return nil
	default:
		return fmt.Errorf("unknown pipeline command: %s", args[0])
	}
}

func printPipelineUsage() {
	fmt.Print(`caged pipeline — manage pipelines

Usage: caged pipeline <command> [options]

Commands:
  create    Create a new pipeline from JSON
  list      List all pipelines
  get       Get pipeline details
  delete    Delete a pipeline
  run       Start a pipeline run
  runs      List runs for a pipeline
  cancel    Cancel a running pipeline

Examples:
  caged pipeline create -f pipeline.json
  caged pipeline list
  caged pipeline run <pipeline-id> --repo https://github.com/org/repo
  caged pipeline runs <pipeline-id>
  caged pipeline cancel <pipeline-id> <run-id>
`)
}

func cmdPipelineCreate(args []string) error {
	fs := flag.NewFlagSet("pipeline create", flag.ExitOnError)
	file := fs.String("f", "", "Pipeline definition JSON file (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *file == "" {
		return fmt.Errorf("pipeline definition file required (-f)")
	}

	data, err := os.ReadFile(*file)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	var req api.CreatePipelineRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return fmt.Errorf("parsing JSON: %w", err)
	}

	client, err := mustClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	pipeline, err := client.CreatePipeline(ctx, &req)
	if err != nil {
		return fmt.Errorf("creating pipeline: %w", err)
	}

	fmt.Printf("Created pipeline: %s (%s)\n", pipeline.Name, pipeline.ID)
	return nil
}

func cmdPipelineList(args []string) error {
	client, err := mustClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	pipelines, err := client.ListPipelines(ctx)
	if err != nil {
		return fmt.Errorf("listing pipelines: %w", err)
	}

	if len(pipelines) == 0 {
		fmt.Println("No pipelines found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTAGES\tCREATED")
	for _, p := range pipelines {
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", p.ID, p.Name, len(p.Stages), p.CreatedAt)
	}
	w.Flush()
	return nil
}

func cmdPipelineGet(args []string) error {
	fs := flag.NewFlagSet("pipeline get", flag.ExitOnError)
	outputJSON := fs.Bool("json", false, "Output as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("pipeline ID required")
	}
	pipelineID := fs.Arg(0)

	client, err := mustClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	pipeline, err := client.GetPipeline(ctx, pipelineID)
	if err != nil {
		return fmt.Errorf("getting pipeline: %w", err)
	}

	if *outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(pipeline)
	}

	fmt.Printf("Pipeline: %s\n", pipeline.Name)
	fmt.Printf("ID: %s\n", pipeline.ID)
	if pipeline.Description != "" {
		fmt.Printf("Description: %s\n", pipeline.Description)
	}
	fmt.Printf("Created: %s\n", pipeline.CreatedAt)
	fmt.Printf("\nStages (%d):\n", len(pipeline.Stages))
	for i, s := range pipeline.Stages {
		deps := ""
		if len(s.DependsOn) > 0 {
			deps = fmt.Sprintf(" (after: %s)", strings.Join(s.DependsOn, ", "))
		}
		fmt.Printf("  %d. %s [%s]%s\n", i+1, s.Name, s.Type, deps)
	}
	return nil
}

func cmdPipelineDelete(args []string) error {
	fs := flag.NewFlagSet("pipeline delete", flag.ExitOnError)
	force := fs.Bool("force", false, "Skip confirmation")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("pipeline ID required")
	}
	pipelineID := fs.Arg(0)

	if !*force {
		fmt.Printf("Delete pipeline %s? [y/N] ", pipelineID)
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" {
			fmt.Println("Aborted")
			return nil
		}
	}

	client, err := mustClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	if err := client.DeletePipeline(ctx, pipelineID); err != nil {
		return fmt.Errorf("deleting pipeline: %w", err)
	}

	fmt.Println("Pipeline deleted")
	return nil
}

func cmdPipelineRun(args []string) error {
	fs := flag.NewFlagSet("pipeline run", flag.ExitOnError)
	repo := fs.String("repo", "", "Git repository to clone")
	branch := fs.String("branch", "", "Git branch to checkout")
	envStr := fs.String("env", "", "Environment variables (KEY=VAL,KEY2=VAL2)")
	trigger := fs.String("trigger", "cli", "Trigger source (cli, api, webhook, etc.)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("pipeline ID required")
	}
	pipelineID := fs.Arg(0)

	client, err := mustClient()
	if err != nil {
		return err
	}

	req := &api.StartRunRequest{
		Trigger: *trigger,
		Repo:    *repo,
		Branch:  *branch,
	}
	if *envStr != "" {
		req.Env = parseEnvVars(*envStr)
	}

	ctx := context.Background()
	run, err := client.StartRun(ctx, pipelineID, req)
	if err != nil {
		return fmt.Errorf("starting run: %w", err)
	}

	fmt.Printf("Started run: %s\n", run.ID)
	fmt.Printf("Status: %s\n", run.Status)
	fmt.Printf("\nView run: caged pipeline runs %s\n", pipelineID)
	return nil
}

func cmdPipelineRuns(args []string) error {
	fs := flag.NewFlagSet("pipeline runs", flag.ExitOnError)
	outputJSON := fs.Bool("json", false, "Output as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("pipeline ID required")
	}
	pipelineID := fs.Arg(0)

	// If a second arg is given, get a specific run.
	if fs.NArg() >= 2 {
		return cmdPipelineGetRun(pipelineID, fs.Arg(1), *outputJSON)
	}

	client, err := mustClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	runs, err := client.ListRuns(ctx, pipelineID)
	if err != nil {
		return fmt.Errorf("listing runs: %w", err)
	}

	if *outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(runs)
	}

	if len(runs) == 0 {
		fmt.Println("No runs found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "RUN ID\tSTATUS\tTRIGGER\tSTARTED\tENDED")
	for _, r := range runs {
		ended := r.EndedAt
		if ended == "" {
			ended = "-"
		}
		started := r.StartedAt
		if started == "" {
			started = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.ID, r.Status, r.Trigger, started, ended)
	}
	w.Flush()
	return nil
}

func cmdPipelineGetRun(pipelineID, runID string, outputJSON bool) error {
	client, err := mustClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	run, err := client.GetRun(ctx, pipelineID, runID)
	if err != nil {
		return fmt.Errorf("getting run: %w", err)
	}

	if outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(run)
	}

	fmt.Printf("Run ID: %s\n", run.ID)
	fmt.Printf("Pipeline: %s\n", run.PipelineID)
	fmt.Printf("Status: %s\n", run.Status)
	fmt.Printf("Trigger: %s\n", run.Trigger)
	if run.StartedAt != "" {
		fmt.Printf("Started: %s\n", run.StartedAt)
	}
	if run.EndedAt != "" {
		fmt.Printf("Ended: %s\n", run.EndedAt)
	}
	return nil
}

func cmdPipelineCancel(args []string) error {
	fs := flag.NewFlagSet("pipeline cancel", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 2 {
		return fmt.Errorf("usage: caged pipeline cancel <pipeline-id> <run-id>")
	}
	pipelineID := fs.Arg(0)
	runID := fs.Arg(1)

	client, err := mustClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	if err := client.CancelRun(ctx, pipelineID, runID); err != nil {
		return fmt.Errorf("canceling run: %w", err)
	}

	fmt.Println("Run canceled")
	return nil
}
