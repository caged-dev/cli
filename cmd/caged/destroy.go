package main

import (
	"context"
	"fmt"
)

func cmdDestroy(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: caged destroy <sandbox-id>")
	}
	sandboxID := args[0]

	client, err := mustClient()
	if err != nil {
		return err
	}

	if err := client.DestroySandbox(context.Background(), sandboxID); err != nil {
		return fmt.Errorf("destroying sandbox: %w", err)
	}

	fmt.Printf("Sandbox %s destroyed.\n", sandboxID)
	return nil
}
