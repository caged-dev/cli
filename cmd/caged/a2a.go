package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/caged-dev/cli/internal/api"
)

func cmdA2A(args []string) error {
	if len(args) == 0 {
		printA2AUsage()
		return nil
	}

	switch args[0] {
	case "agents":
		return cmdA2AAgents(args[1:])
	case "discover":
		return cmdA2ADiscover(args[1:])
	case "delegate":
		return cmdA2ADelegate(args[1:])
	case "task":
		return cmdA2ATask(args[1:])
	case "help", "--help", "-h":
		printA2AUsage()
		return nil
	default:
		return fmt.Errorf("unknown a2a command: %s", args[0])
	}
}

func printA2AUsage() {
	fmt.Print(`caged a2a — Agent-to-Agent protocol commands

Usage: caged a2a <command> [options]

Commands:
  agents     Manage A2A agent registrations
  discover   Discover remote A2A agents
  delegate   Delegate a task to an A2A agent
  task       Manage A2A tasks

Examples:
  caged a2a agents list
  caged a2a agents create -f agent.json
  caged a2a discover https://agent.example.com
  caged a2a delegate https://agent.example.com -skill analyze -input '{"code": "..."}'
  caged a2a task get <task-id>
`)
}

// ============================================================================
// Agents subcommands
// ============================================================================

func cmdA2AAgents(args []string) error {
	if len(args) == 0 {
		return cmdA2AAgentsList(nil)
	}

	switch args[0] {
	case "list", "ls":
		return cmdA2AAgentsList(args[1:])
	case "create":
		return cmdA2AAgentsCreate(args[1:])
	case "get":
		return cmdA2AAgentsGet(args[1:])
	case "delete", "rm":
		return cmdA2AAgentsDelete(args[1:])
	case "enable":
		return cmdA2AAgentsToggle(args[1:], true)
	case "disable":
		return cmdA2AAgentsToggle(args[1:], false)
	default:
		return fmt.Errorf("unknown agents command: %s", args[0])
	}
}

func cmdA2AAgentsList(_ []string) error {
	client, err := mustClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	agents, err := client.ListA2AAgents(ctx)
	if err != nil {
		return fmt.Errorf("listing agents: %w", err)
	}

	if len(agents) == 0 {
		fmt.Println("No A2A agents registered.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tPUBLIC\tENABLED\tSKILLS")
	for _, a := range agents {
		skillCount := len(a.Skills)
		fmt.Fprintf(w, "%s\t%s\t%v\t%v\t%d\n",
			a.ID, a.Name, a.Public, a.Enabled, skillCount)
	}
	w.Flush()

	return nil
}

func cmdA2AAgentsCreate(args []string) error {
	fs := flag.NewFlagSet("a2a agents create", flag.ExitOnError)
	file := fs.String("f", "", "Agent definition JSON file (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *file == "" {
		return fmt.Errorf("agent definition file required (-f)")
	}

	data, err := os.ReadFile(*file)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	var req api.CreateA2AAgentRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return fmt.Errorf("parsing JSON: %w", err)
	}

	client, err := mustClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	agent, err := client.CreateA2AAgent(ctx, &req)
	if err != nil {
		return fmt.Errorf("creating agent: %w", err)
	}

	fmt.Printf("Created A2A agent: %s (%s)\n", agent.Name, agent.ID)
	return nil
}

func cmdA2AAgentsGet(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: caged a2a agents get <agent-id>")
	}

	client, err := mustClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	agent, err := client.GetA2AAgent(ctx, args[0])
	if err != nil {
		return fmt.Errorf("getting agent: %w", err)
	}

	// Pretty-print JSON.
	data, _ := json.MarshalIndent(agent, "", "  ")
	fmt.Println(string(data))
	return nil
}

func cmdA2AAgentsDelete(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: caged a2a agents delete <agent-id>")
	}

	client, err := mustClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	if err := client.DeleteA2AAgent(ctx, args[0]); err != nil {
		return fmt.Errorf("deleting agent: %w", err)
	}

	fmt.Println("Agent deleted.")
	return nil
}

func cmdA2AAgentsToggle(args []string, enable bool) error {
	if len(args) < 1 {
		action := "enable"
		if !enable {
			action = "disable"
		}
		return fmt.Errorf("usage: caged a2a agents %s <agent-id>", action)
	}

	client, err := mustClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	req := &api.UpdateA2AAgentRequest{Enabled: &enable}
	if _, err := client.UpdateA2AAgent(ctx, args[0], req); err != nil {
		return fmt.Errorf("updating agent: %w", err)
	}

	if enable {
		fmt.Println("Agent enabled.")
	} else {
		fmt.Println("Agent disabled.")
	}
	return nil
}

// ============================================================================
// Discover subcommand
// ============================================================================

func cmdA2ADiscover(args []string) error {
	fs := flag.NewFlagSet("a2a discover", flag.ExitOnError)
	outputJSON := fs.Bool("json", false, "Output as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: caged a2a discover <agent-url>")
	}

	agentURL := fs.Arg(0)

	client, err := mustClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	card, err := client.DiscoverA2AAgent(ctx, agentURL)
	if err != nil {
		return fmt.Errorf("discovering agent: %w", err)
	}

	if *outputJSON {
		data, _ := json.MarshalIndent(card, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Agent: %s\n", card.Name)
	fmt.Printf("  Description: %s\n", card.Description)
	fmt.Printf("  URL: %s\n", card.URL)
	fmt.Printf("  Protocol: %s\n", card.Version)
	fmt.Printf("  Streaming: %s\n", card.StreamingMode)

	if len(card.Skills) > 0 {
		fmt.Println("  Skills:")
		for _, skill := range card.Skills {
			fmt.Printf("    - %s: %s\n", skill.ID, skill.Name)
		}
	}

	if card.Provider != nil {
		fmt.Printf("  Provider: %s (%s)\n", card.Provider.Name, card.Provider.Organization)
	}

	return nil
}

// ============================================================================
// Delegate subcommand
// ============================================================================

func cmdA2ADelegate(args []string) error {
	fs := flag.NewFlagSet("a2a delegate", flag.ExitOnError)
	skill := fs.String("skill", "", "Skill ID to invoke")
	input := fs.String("input", "", "Task input JSON")
	inputFile := fs.String("input-file", "", "Task input JSON file")
	stream := fs.Bool("stream", false, "Stream task updates")
	wait := fs.Bool("wait", true, "Wait for task completion")
	outputJSON := fs.Bool("json", false, "Output as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: caged a2a delegate <agent-url> [options]")
	}

	agentURL := fs.Arg(0)

	// Build input.
	var inputData json.RawMessage
	if *inputFile != "" {
		data, err := os.ReadFile(*inputFile)
		if err != nil {
			return fmt.Errorf("reading input file: %w", err)
		}
		inputData = data
	} else if *input != "" {
		inputData = json.RawMessage(*input)
	}

	client, err := mustClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	req := &api.CreateA2ATaskRequest{
		SkillID:   *skill,
		Input:     inputData,
		Streaming: *stream,
	}

	task, err := client.CreateA2ATask(ctx, agentURL, req)
	if err != nil {
		return fmt.Errorf("creating task: %w", err)
	}

	fmt.Printf("Created task: %s (status: %s)\n", task.ID, task.Status)

	if *wait && task.Status != "completed" && task.Status != "failed" && task.Status != "canceled" {
		fmt.Println("Waiting for task completion...")
		task, err = client.WaitA2ATask(ctx, agentURL, task.ID)
		if err != nil {
			return fmt.Errorf("waiting for task: %w", err)
		}
	}

	if *outputJSON {
		data, _ := json.MarshalIndent(task, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Task %s: %s\n", task.ID, task.Status)
	if task.Output != nil {
		fmt.Printf("Output: %s\n", string(task.Output))
	}
	if task.StatusMessage != "" {
		fmt.Printf("Message: %s\n", task.StatusMessage)
	}

	return nil
}

// ============================================================================
// Task subcommand
// ============================================================================

func cmdA2ATask(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: caged a2a task <get|cancel|messages> [options]")
	}

	switch args[0] {
	case "get":
		return cmdA2ATaskGet(args[1:])
	case "cancel":
		return cmdA2ATaskCancel(args[1:])
	case "message":
		return cmdA2ATaskMessage(args[1:])
	default:
		return fmt.Errorf("unknown task command: %s", args[0])
	}
}

func cmdA2ATaskGet(args []string) error {
	fs := flag.NewFlagSet("a2a task get", flag.ExitOnError)
	agentURL := fs.String("agent", "", "Agent URL (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 || *agentURL == "" {
		return fmt.Errorf("usage: caged a2a task get <task-id> -agent <agent-url>")
	}

	taskID := fs.Arg(0)

	client, err := mustClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	task, err := client.GetA2ATask(ctx, *agentURL, taskID)
	if err != nil {
		return fmt.Errorf("getting task: %w", err)
	}

	data, _ := json.MarshalIndent(task, "", "  ")
	fmt.Println(string(data))
	return nil
}

func cmdA2ATaskCancel(args []string) error {
	fs := flag.NewFlagSet("a2a task cancel", flag.ExitOnError)
	agentURL := fs.String("agent", "", "Agent URL (required)")
	reason := fs.String("reason", "", "Cancellation reason")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 || *agentURL == "" {
		return fmt.Errorf("usage: caged a2a task cancel <task-id> -agent <agent-url>")
	}

	taskID := fs.Arg(0)

	client, err := mustClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	req := &api.CancelA2ATaskRequest{Reason: *reason}
	if err := client.CancelA2ATask(ctx, *agentURL, taskID, req); err != nil {
		return fmt.Errorf("canceling task: %w", err)
	}

	fmt.Println("Task canceled.")
	return nil
}

func cmdA2ATaskMessage(args []string) error {
	fs := flag.NewFlagSet("a2a task message", flag.ExitOnError)
	agentURL := fs.String("agent", "", "Agent URL (required)")
	text := fs.String("text", "", "Message text")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 || *agentURL == "" || *text == "" {
		return fmt.Errorf("usage: caged a2a task message <task-id> -agent <agent-url> -text <message>")
	}

	taskID := fs.Arg(0)

	client, err := mustClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	req := &api.SendA2AMessageRequest{
		Parts: []api.A2APart{
			{Type: "text", Text: *text},
		},
	}
	msg, err := client.SendA2AMessage(ctx, *agentURL, taskID, req)
	if err != nil {
		return fmt.Errorf("sending message: %w", err)
	}

	fmt.Printf("Message sent: %s\n", msg.ID)
	return nil
}
