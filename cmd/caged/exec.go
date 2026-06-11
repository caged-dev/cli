package main

import (
	"context"
	"fmt"
	"os"
	"strings"
)

func cmdExec(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: caged exec <sandbox-id> <command>")
	}
	sandboxID := args[0]
	command := strings.Join(args[1:], " ")

	client, err := mustClient()
	if err != nil {
		return err
	}

	output, exitCode, err := client.Exec(context.Background(), sandboxID, command)
	if output != "" {
		fmt.Print(output)
		if !strings.HasSuffix(output, "\n") {
			fmt.Println()
		}
	}
	if err != nil {
		return fmt.Errorf("exec: %w", err)
	}
	if exitCode != 0 {
		// Propagate the remote command's exit code, like ssh does.
		os.Exit(exitCode)
	}
	return nil
}
