package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
)

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	format := fs.String("format", "table", "Output format: table, json")
	if err := fs.Parse(args); err != nil {
		return err
	}

	client, err := mustClient()
	if err != nil {
		return err
	}

	sandboxes, err := client.ListSandboxes(context.Background())
	if err != nil {
		return fmt.Errorf("listing sandboxes: %w", err)
	}

	switch *format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(sandboxes)

	case "table":
		if len(sandboxes) == 0 {
			fmt.Println("No sandboxes found.")
			fmt.Println("Create one with: caged sandboxes create --template node")
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

	default:
		return fmt.Errorf("unknown format: %s (use 'table' or 'json')", *format)
	}
}
