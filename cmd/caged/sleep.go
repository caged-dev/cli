package main

import (
	"context"
	"fmt"
)

func cmdSleep(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: caged sleep <sandbox-id>")
	}
	sandboxID := args[0]

	client, err := mustClient()
	if err != nil {
		return err
	}

	if err := client.SleepSandbox(context.Background(), sandboxID); err != nil {
		return fmt.Errorf("sleeping sandbox: %w", err)
	}

	fmt.Printf("Sandbox %s is now sleeping. No compute charges while paused.\n", sandboxID)
	return nil
}

func cmdWake(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: caged wake <sandbox-id>")
	}
	sandboxID := args[0]

	client, err := mustClient()
	if err != nil {
		return err
	}

	if err := client.WakeSandbox(context.Background(), sandboxID); err != nil {
		return fmt.Errorf("waking sandbox: %w", err)
	}

	fmt.Printf("Sandbox %s is awake and running.\n", sandboxID)
	return nil
}
