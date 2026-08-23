package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/saugatadhikari/jobSync/internal/cloud/client"
	"github.com/saugatadhikari/jobSync/internal/config"
	"github.com/saugatadhikari/jobSync/internal/google/auth"
)

func runCloud(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(`usage:
  jobsync cloud push   Register for hosted daily sync`)
	}
	switch strings.ToLower(args[0]) {
	case "push":
		return runCloudPush(args[1:])
	case "help", "-h", "--help":
		fmt.Println(`Usage:
  jobsync cloud push [--server URL]`)
		return nil
	default:
		return fmt.Errorf("unknown cloud subcommand %q", args[0])
	}
}

func runCloudPush(args []string) error {
	serverURL := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--server":
			if i+1 >= len(args) {
				return fmt.Errorf("--server requires a value")
			}
			i++
			serverURL = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if !cfg.HasGeminiKey() {
		return fmt.Errorf("no Gemini API key — run jobsync init first")
	}
	if cfg.SpreadsheetID == "" {
		return fmt.Errorf("no spreadsheet configured — run jobsync init first")
	}

	serverURL = resolveCloudServerURL(serverURL, cfg)
	if serverURL == "" {
		return fmt.Errorf("cloud server URL not configured — use a release binary or pass --server")
	}

	tokenPath, err := config.TokenPath()
	if err != nil {
		return err
	}
	tokenJSON, err := os.ReadFile(tokenPath)
	if err != nil {
		return fmt.Errorf("read oauth token: %w (run jobsync init first)", err)
	}
	if _, err := auth.TokenFromJSON(tokenJSON); err != nil {
		return fmt.Errorf("invalid oauth token: %w", err)
	}

	ctx := context.Background()
	// Refresh token locally (and verify Google auth) before sending to the server.
	if _, err := auth.GoogleEmailFromTokenJSON(ctx, tokenJSON); err != nil {
		return fmt.Errorf("google sign-in expired — run jobsync init: %w", err)
	}
	tokenJSON, err = os.ReadFile(tokenPath)
	if err != nil {
		return fmt.Errorf("read oauth token: %w", err)
	}

	resp, err := client.Register(ctx, serverURL, client.RegisterRequest{
		SpreadsheetID:     cfg.SpreadsheetID,
		SheetName:         cfg.SheetName,
		GeminiAPIKey:      cfg.GeminiAPIKey,
		GeminiModel:       cfg.GeminiModel,
		OAuthTokenJSON:    string(tokenJSON),
		AuthScopesVersion: cfg.AuthScopesVersion,
	})
	if err != nil {
		return err
	}

	cfg.CloudSyncEnabled = true
	cfg.CloudAccountID = resp.AccountID
	cfg.CloudServerURL = serverURL
	if err := config.Save(cfg); err != nil {
		return err
	}
	if err := removeLegacyLocalSchedule(); err != nil {
		fmt.Printf("Warning: could not remove old local schedule: %v\n", err)
	}

	fmt.Printf("Registered for cloud sync (account %q)\n", resp.AccountID)
	fmt.Println("Daily sync runs on the server — no Mac required.")
	return nil
}

func resolveCloudServerURL(flag string, cfg *config.Config) string {
	if v := strings.TrimRight(strings.TrimSpace(flag), "/"); v != "" {
		return v
	}
	if v := strings.TrimRight(strings.TrimSpace(os.Getenv("JOBSYNC_CLOUD_URL")), "/"); v != "" {
		return v
	}
	if cfg != nil {
		if v := strings.TrimRight(strings.TrimSpace(cfg.CloudServerURL), "/"); v != "" {
			return v
		}
	}
	return strings.TrimRight(strings.TrimSpace(config.DefaultCloudServerURL), "/")
}

const (
	legacyLaunchdLabel = "com.jobsync.sync"
	legacyCronMarker   = "# jobsync daily sync"
)

func removeLegacyLocalSchedule() error {
	switch runtime.GOOS {
	case "darwin":
		return removeLegacyLaunchd()
	case "linux":
		return removeLegacyCron()
	default:
		return nil
	}
}

func removeLegacyLaunchd() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(home, "Library", "LaunchAgents", legacyLaunchdLabel+".plist")
	domain := fmt.Sprintf("gui/%d/%s", os.Getuid(), legacyLaunchdLabel)
	_ = exec.Command("launchctl", "bootout", domain).Run()
	_ = exec.Command("launchctl", "unload", path).Run()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func removeLegacyCron() error {
	out, err := exec.Command("crontab", "-l").Output()
	if err != nil {
		return nil
	}
	var kept []string
	removed := false
	for _, l := range strings.Split(string(out), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if strings.Contains(l, legacyCronMarker) || strings.Contains(l, "jobsync sync") {
			removed = true
			continue
		}
		kept = append(kept, l)
	}
	if !removed {
		return nil
	}
	if len(kept) == 0 {
		return exec.Command("crontab", "-r").Run()
	}
	var buf bytes.Buffer
	for _, l := range kept {
		buf.WriteString(l)
		buf.WriteByte('\n')
	}
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = &buf
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("crontab: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}
