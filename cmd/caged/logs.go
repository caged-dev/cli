package main

import (
	"context"
	"flag"
	"fmt"
)

func cmdLogs(args []string) error {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	follow := fs.Bool("f", false, "Follow log output")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) < 1 {
		return fmt.Errorf("usage: caged logs [-f] <sandbox-id>")
	}
	sandboxID := remaining[0]

	client, err := mustClient()
	if err != nil {
		return err
	}

	logs, err := client.GetLogs(context.Background(), sandboxID, *follow)
	if err != nil {
		return fmt.Errorf("getting logs: %w", err)
	}

	if len(logs) == 0 {
		fmt.Println("No logs yet.")
		return nil
	}

	for _, entry := range logs {
		fmt.Printf("%s [%s] %s\n", entry.Timestamp, entry.Type, entry.Message)
	}
	return nil
}
