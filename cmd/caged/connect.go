package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sandbox, err := client.GetSandbox(ctx, sandboxID)
	if err != nil {
		return fmt.Errorf("getting sandbox: %w", err)
	}
	if sandbox.Status != "running" {
		return fmt.Errorf("sandbox %s is %s (must be running)", sandboxID, sandbox.Status)
	}

	stdinFD := int(os.Stdin.Fd())
	if !term.IsTerminal(stdinFD) {
		return fmt.Errorf("caged connect requires an interactive terminal (use 'caged exec' for scripts)")
	}

	cols, rows, err := term.GetSize(stdinFD)
	if err != nil {
		cols, rows = 80, 24
	}

	tc, err := client.ConnectTerminal(ctx, sandboxID, rows, cols)
	if err != nil {
		return err
	}
	defer func() { _ = tc.Close() }()

	fmt.Printf("Connected to sandbox %s (%s). Type 'exit' or press Ctrl+D to disconnect.\r\n", sandboxID, sandbox.Template)

	// Raw mode: pass every keystroke (incl. Ctrl+C, arrows, tab) to the sandbox.
	oldState, err := term.MakeRaw(stdinFD)
	if err != nil {
		return fmt.Errorf("entering raw mode: %w", err)
	}
	defer func() {
		_ = term.Restore(stdinFD, oldState)
		fmt.Println()
	}()

	// Propagate local terminal resizes to the remote PTY.
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	defer signal.Stop(winch)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-winch:
				if c, r, sErr := term.GetSize(stdinFD); sErr == nil {
					_ = tc.Resize(ctx, uint16(r), uint16(c))
				}
			}
		}
	}()

	// Local stdin → remote PTY.
	go func() {
		defer cancel()
		buf := make([]byte, 4096)
		for {
			n, rErr := os.Stdin.Read(buf)
			if n > 0 {
				if wErr := tc.WriteInput(ctx, buf[:n]); wErr != nil {
					return
				}
			}
			if rErr != nil {
				return
			}
		}
	}()

	// Remote PTY → local stdout.
	for {
		out, rErr := tc.Read(ctx)
		if rErr != nil {
			return nil // Session ended (shell exit, sandbox destroyed, or disconnect).
		}
		if _, wErr := os.Stdout.Write(out); wErr != nil {
			return nil
		}
	}
}
