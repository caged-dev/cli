package main

import (
	"context"
	"fmt"
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

	output, err := client.Exec(context.Background(), sandboxID, command)
	if err != nil {
		return fmt.Errorf("exec: %w", err)
	}
	if output != "" {
		fmt.Print(output)
		if !strings.HasSuffix(output, "\n") {
			fmt.Println()
		}
	}
	return nil
}
