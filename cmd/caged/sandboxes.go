package main

import (
	"fmt"
)

func cmdSandboxes(args []string) error {
	if len(args) == 0 {
		printSandboxesUsage()
		return nil
	}

	switch args[0] {
	case "list", "ls":
		return cmdList(args[1:])
	case "create":
		return cmdRun(args[1:])
	case "destroy", "rm":
		if len(args) < 2 {
			return fmt.Errorf("usage: caged sandboxes destroy <sandbox-id>")
		}
		return cmdDestroy(args[1:])
	case "sleep":
		if len(args) < 2 {
			return fmt.Errorf("usage: caged sandboxes sleep <sandbox-id>")
		}
		return cmdSleep(args[1:])
	case "wake":
		if len(args) < 2 {
			return fmt.Errorf("usage: caged sandboxes wake <sandbox-id>")
		}
		return cmdWake(args[1:])
	case "logs":
		if len(args) < 2 {
			return fmt.Errorf("usage: caged sandboxes logs <sandbox-id>")
		}
		return cmdLogs(args[1:])
	case "connect":
		if len(args) < 2 {
			return fmt.Errorf("usage: caged sandboxes connect <sandbox-id>")
		}
		return cmdConnect(args[1:])
	case "exec":
		if len(args) < 3 {
			return fmt.Errorf("usage: caged sandboxes exec <sandbox-id> <command>")
		}
		return cmdExec(args[1:])
	case "help", "--help", "-h":
		printSandboxesUsage()
		return nil
	default:
		return fmt.Errorf("unknown sandboxes command: %s (run 'caged sandboxes help' for usage)", args[0])
	}
}

func printSandboxesUsage() {
	fmt.Printf(`caged sandboxes — Manage sandboxes

Usage: caged sandboxes <command> [options]

Commands:
  list (ls)           List all sandboxes
  create              Create a new sandbox
  destroy (rm) <id>   Destroy a sandbox
  sleep <id>          Pause a running sandbox
  wake <id>           Resume a sleeping sandbox
  logs <id>           View sandbox logs
  connect <id>        Interactive terminal session
  exec <id> <cmd>     Execute command in sandbox

Examples:
  caged sandboxes list
  caged sandboxes list --format json
  caged sandboxes create --template node
  caged sandboxes connect cage_abc123
  caged sandboxes destroy cage_abc123

Shortcuts (backward compatibility):
  caged list        → caged sandboxes list
  caged run         → caged sandboxes create
  caged connect     → caged sandboxes connect
  caged destroy     → caged sandboxes destroy

`)
}
