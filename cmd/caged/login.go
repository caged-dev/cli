package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/caged-dev/cli/internal/config"
)

func cmdLogin(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("API URL [%s]: ", cfg.APIURL)
	urlInput, _ := reader.ReadString('\n')
	urlInput = strings.TrimSpace(urlInput)
	if urlInput != "" {
		cfg.APIURL = urlInput
	}

	fmt.Print("API Key (caged_...): ")
	keyInput, _ := reader.ReadString('\n')
	keyInput = strings.TrimSpace(keyInput)
	if keyInput == "" {
		return fmt.Errorf("API key is required")
	}

	if !strings.HasPrefix(keyInput, "caged_") {
		return fmt.Errorf("invalid API key format; must start with 'caged_'")
	}

	cfg.APIKey = keyInput

	if err := config.Save(cfg); err != nil {
		return err
	}

	path, _ := config.Path()
	fmt.Printf("Credentials saved to %s\n", path)
	return nil
}
