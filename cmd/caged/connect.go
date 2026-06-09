package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func cmdConnect(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: caged connect <sandbox-id>")
	}
	sandboxID := args[0]

	client, err := mustClient()
	if err != nil {
		return err
	}

	sandbox, err := client.GetSandbox(context.Background(), sandboxID)
	if err != nil {
		return fmt.Errorf("getting sandbox: %w", err)
	}
	if sandbox.Status != "running" {
		return fmt.Errorf("sandbox %s is %s (must be running)", sandboxID, sandbox.Status)
	}

	fmt.Printf("Connected to sandbox %s (%s)\n", sandboxID, sandbox.Template)
	fmt.Printf("Type commands to execute. Press Ctrl+C or type 'exit' to disconnect.\n\n")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("caged> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println()
				return nil
			}
			return err
		}

		cmd := strings.TrimSpace(line)
		if cmd == "" {
			continue
		}
		if cmd == "exit" || cmd == "quit" {
			return nil
		}

		output, execErr := client.Exec(ctx, sandboxID, cmd)
		if execErr != nil {
			fmt.Fprintf(os.Stderr, "exec error: %v\n", execErr)
			continue
		}
		if output != "" {
			fmt.Println(output)
		}
	}
}
