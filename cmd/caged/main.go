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
		return fmt.Errorf("unknown command: %s\nRun 'caged help' for usage.", os.Args[1])
	}
}

func printUsage() {
	fmt.Printf(`caged %s — AI Agent Sandbox Platform
https://caged.dev

Usage: caged <command> [options]

Commands:
  login               Configure API credentials
  up                  Create sandbox from .caged.yaml
  run                 Create and start a sandbox
  list (ls)           List sandboxes
  connect <id>        Interactive terminal session
  exec <id> <cmd>     Execute a command in a sandbox
  destroy (rm) <id>   Destroy a sandbox
  sleep <id>          Pause sandbox (saves costs)
  wake <id>           Resume a sleeping sandbox
  logs <id>           Stream sandbox events
  version             Show version

Examples:
  caged login
  caged run --template node-20 --cpus 2 --memory 1024
  caged up
  caged connect cage_abc123
  caged exec cage_abc123 "npm test"
  caged destroy cage_abc123

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
