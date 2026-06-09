package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
)

func cmdList(args []string) error {
	client, err := mustClient()
	if err != nil {
		return err
	}

	sandboxes, err := client.ListSandboxes(context.Background())
	if err != nil {
		return fmt.Errorf("listing sandboxes: %w", err)
	}

	if len(sandboxes) == 0 {
		fmt.Println("No sandboxes found.")
		fmt.Println("Create one with: caged run --template node-20")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID\tSTATUS\tTEMPLATE\tCPUs\tMEMORY\tCREATED\n")
	for _, s := range sandboxes {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%dMB\t%s\n",
			s.ID, s.Status, s.Template, s.CPUs, s.MemoryMB, s.CreatedAt)
	}
	w.Flush()
	return nil
}
