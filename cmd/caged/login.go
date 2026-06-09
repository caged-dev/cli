package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/caged-dev/cli/internal/config"
)

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type deviceTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	AccountID   string `json:"account_id"`
	Email       string `json:"email"`
}

type deviceTokenError struct {
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

func cmdLogin(args []string) error {
	// Check for --manual flag to use the old interactive flow.
	for _, arg := range args {
		if arg == "--manual" {
			return cmdLoginManual()
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Println("Logging in to Caged...")
	fmt.Println()

	// Step 1: Request a device code from the API.
	reqBody, _ := json.Marshal(struct{}{})
	resp, err := http.Post(cfg.APIURL+"/v1/auth/device/code", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("connecting to %s: %w\n\nUse 'caged login --manual' to enter an API key directly", cfg.APIURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d — use 'caged login --manual' to enter an API key directly", resp.StatusCode)
	}

	var codeResp deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&codeResp); err != nil {
		return fmt.Errorf("parsing device code response: %w", err)
	}

	// Step 2: Show the user code and open the browser.
	verifyURL := codeResp.VerificationURL + "?code=" + codeResp.UserCode
	fmt.Printf("  Your code: %s\n", codeResp.UserCode)
	fmt.Println()
	fmt.Printf("  Opening browser to: %s\n", verifyURL)
	fmt.Println()
	fmt.Println("  Waiting for authorization...")

	// Try to open the browser.
	if err := openBrowser(verifyURL); err != nil {
		fmt.Printf("  Could not open browser. Please visit the URL manually:\n")
		fmt.Printf("  %s\n\n", verifyURL)
	}

	// Step 3: Poll for the token.
	interval := time.Duration(codeResp.Interval) * time.Second
	if interval < 3*time.Second {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(codeResp.ExpiresIn) * time.Second)

	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("login timed out — please try again")
		case <-time.After(interval):
		}

		tokenResp, status, pollErr := pollDeviceToken(cfg.APIURL, codeResp.DeviceCode)
		if pollErr != nil {
			return fmt.Errorf("polling for token: %w", pollErr)
		}

		switch status {
		case "authorization_pending":
			continue
		case "success":
			cfg.APIKey = tokenResp.AccessToken
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Println()
			fmt.Printf("  Logged in as %s\n", tokenResp.Email)
			path, _ := config.Path()
			fmt.Printf("  Credentials saved to %s\n", path)
			return nil
		case "access_denied":
			return fmt.Errorf("authorization denied")
		case "expired_token":
			return fmt.Errorf("device code expired — please try again")
		default:
			return fmt.Errorf("unexpected response: %s", status)
		}
	}
}

func pollDeviceToken(apiURL, deviceCode string) (*deviceTokenResponse, string, error) {
	body, _ := json.Marshal(map[string]string{"device_code": deviceCode})
	resp, err := http.Post(apiURL+"/v1/auth/device/token", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var tokenResp deviceTokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
			return nil, "", err
		}
		return &tokenResp, "success", nil
	}

	var errResp deviceTokenError
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		return nil, "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil, errResp.Error, nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}
	return cmd.Start()
}

// cmdLoginManual is the fallback manual API key entry flow.
func cmdLoginManual() error {
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
