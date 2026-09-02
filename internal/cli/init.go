package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/saugatadhikari/jobSync/internal/cloud/client"
	"github.com/saugatadhikari/jobSync/internal/config"
	"github.com/saugatadhikari/jobSync/internal/google/auth"
	"github.com/saugatadhikari/jobSync/internal/google/gmail"
	"github.com/saugatadhikari/jobSync/internal/google/sheets"
)

func runInit(args []string) error {
	_ = args
	ctx := context.Background()

	dir, err := config.EnsureDir()
	if err != nil {
		return err
	}
	fmt.Printf("config dir: %s\n", dir)
	fmt.Println("Google sign-in uses the built-in JobSync OAuth app.")
	fmt.Println()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	httpClient, err := auth.EnsureScopes(ctx, cfg, auth.RequiredScopes, auth.CurrentScopesVersion)
	if err != nil {
		return err
	}
	fmt.Println("Google sign-in: ok (Sheets + Gmail)")

	if cfg.SpreadsheetID == "" {
		if reused, err := adoptCloudSheet(ctx, cfg); err != nil {
			fmt.Printf("Cloud lookup skipped: %v\n", err)
		} else if reused {
			fmt.Printf("Found your existing cloud sheet for this Gmail.\n")
		}
	}

	if cfg.SpreadsheetID == "" {
		fmt.Println("Creating JobSync tracker spreadsheet...")
		id, err := sheets.CreateTrackerSpreadsheet(ctx, httpClient, "JobSync Tracker", cfg.SheetName)
		if err != nil {
			return err
		}
		cfg.SpreadsheetID = id
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("Created spreadsheet: %s\n", sheets.SpreadsheetURL(id))
	} else {
		fmt.Printf("Using spreadsheet: %s\n", sheets.SpreadsheetURL(cfg.SpreadsheetID))
		client, err := sheets.NewClient(ctx, httpClient, cfg.SpreadsheetID, cfg.SheetName)
		if err != nil {
			return err
		}
		fmt.Println("Refreshing sheet layout...")
		if err := client.SetupSheet(ctx); err != nil {
			return err
		}
	}

	gclient, err := gmail.NewClient(ctx, httpClient)
	if err != nil {
		return err
	}
	msgs, err := gclient.Search(ctx, gmail.DefaultQuery, 5, time.Time{})
	if err != nil {
		return fmt.Errorf("gmail search: %w", err)
	}
	fmt.Printf("Gmail search: ok (%d recent candidate emails)\n", len(msgs))

	if err := ensureGeminiKey(cfg); err != nil {
		return err
	}
	if err := config.Save(cfg); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Init complete.")
	fmt.Printf("Sheet: %s\n", sheets.SpreadsheetURL(cfg.SpreadsheetID))
	if cfg.UsesCloudSync() {
		fmt.Println("Next: already on cloud sync — run jobsync cloud push after credential changes")
	} else {
		fmt.Println("Next: jobsync cloud push")
		fmt.Println("      jobsync sync --dry-run --limit 5   (optional local test)")
	}
	return nil
}

// adoptCloudSheet checks the hosted server for a tracker already linked to this Gmail.
func adoptCloudSheet(ctx context.Context, cfg *config.Config) (bool, error) {
	serverURL := resolveCloudServerURL("", cfg)
	if serverURL == "" {
		return false, fmt.Errorf("no cloud server URL")
	}
	tokenPath, err := config.TokenPath()
	if err != nil {
		return false, err
	}
	tokenJSON, err := os.ReadFile(tokenPath)
	if err != nil {
		return false, err
	}
	out, err := client.Lookup(ctx, serverURL, string(tokenJSON))
	if err != nil {
		return false, err
	}
	if out == nil || !out.Found || strings.TrimSpace(out.SpreadsheetID) == "" {
		return false, nil
	}
	cfg.SpreadsheetID = strings.TrimSpace(out.SpreadsheetID)
	if strings.TrimSpace(out.SheetName) != "" {
		cfg.SheetName = strings.TrimSpace(out.SheetName)
	}
	cfg.CloudAccountID = out.AccountID
	cfg.CloudServerURL = serverURL
	cfg.CloudSyncEnabled = true
	if err := config.Save(cfg); err != nil {
		return false, err
	}
	return true, nil
}

func ensureGeminiKey(cfg *config.Config) error {
	if cfg.HasGeminiKey() {
		fmt.Println("Gemini API key: already saved")
		return nil
	}

	fmt.Println()
	fmt.Println("Gemini setup — open https://aistudio.google.com/apikey")
	fmt.Print("Paste your Gemini API key: ")

	var key string
	if _, err := fmt.Scanln(&key); err != nil {
		return fmt.Errorf("read gemini api key: %w", err)
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("gemini api key is required")
	}

	cfg.GeminiAPIKey = key
	if cfg.GeminiModel == "" {
		cfg.GeminiModel = config.DefaultGeminiModel
	}
	fmt.Println("Gemini API key: saved")
	return nil
}
