package main

import (
	"fmt"
	"os"

	"github.com/caged-dev/cli/internal/api"
	"github.com/caged-dev/cli/internal/config"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		printUsage()
		return nil
	}

	switch os.Args[1] {
	case "login":
		return cmdLogin(os.Args[2:])
	case "up":
		return cmdUp(os.Args[2:])
	case "sandboxes":
		return cmdSandboxes(os.Args[2:])

	// Shortcuts (backward compatibility) — these map to sandboxes subcommands
	case "run":
		return cmdRun(os.Args[2:])
	case "list", "ls":
		return cmdList(os.Args[2:])
	case "connect":
		return cmdConnect(os.Args[2:])
	case "exec":
		return cmdExec(os.Args[2:])
	case "destroy", "rm":
		return cmdDestroy(os.Args[2:])
	case "sleep":
		return cmdSleep(os.Args[2:])
	case "wake":
		return cmdWake(os.Args[2:])
	case "logs":
		return cmdLogs(os.Args[2:])

	case "version", "--version", "-v":
		fmt.Printf("caged %s (%s)\n", version, commit)
		return nil
	case "help", "--help", "-h":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %s (run 'caged help' for usage)", os.Args[1])
	}
}

func printUsage() {
	fmt.Printf(`caged %s — AI Agent Sandbox Platform
https://caged.dev

Usage: caged <command> [options]

Commands:
  login               Configure API credentials
  up                  Create sandbox from .caged.yaml
  sandboxes           Manage sandboxes (list, create, destroy, etc.)
  version             Show version

Sandbox Management:
  caged sandboxes list        List all sandboxes
  caged sandboxes create      Create a new sandbox
  caged sandboxes destroy     Destroy a sandbox
  caged sandboxes sleep       Pause a sandbox
  caged sandboxes wake        Resume a sandbox
  caged sandboxes connect     Interactive terminal
  caged sandboxes exec        Run a command
  caged sandboxes logs        View logs

Shortcuts (backward compatibility):
  caged list          → caged sandboxes list
  caged run           → caged sandboxes create
  caged connect <id>  → caged sandboxes connect <id>
  caged destroy <id>  → caged sandboxes destroy <id>

Examples:
  caged login
  caged up
  caged sandboxes list --format json
  caged sandboxes create --template node --cpus 2
  caged sandboxes connect cage_abc123
  caged sandboxes destroy cage_abc123

`, version)
}

func mustClient() (*api.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("not logged in. Run 'caged login' first")
	}
	return api.NewClient(cfg.APIURL, cfg.APIKey), nil
}
