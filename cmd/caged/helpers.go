package main

import (
	"fmt"

	"github.com/caged-dev/cli/internal/api"
)

// createSandboxRequest is the request body for creating a sandbox.
type createSandboxRequest = api.CreateSandboxRequest

func printSandboxInfo(s *api.Sandbox) {
	fmt.Printf("Sandbox created: %s\n", s.ID)
	fmt.Printf("  Status:   %s\n", s.Status)
	fmt.Printf("  Template: %s\n", s.Template)
	if s.IP != "" {
		fmt.Printf("  IP:       %s\n", s.IP)
	}
	if s.Budget > 0 {
		fmt.Printf("  Budget:   $%.2f\n", s.Budget)
	}
}
