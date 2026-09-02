package cli

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/saugatadhikari/jobSync/internal/config"
	"github.com/saugatadhikari/jobSync/internal/gemini"
	"github.com/saugatadhikari/jobSync/internal/google/auth"
	"github.com/saugatadhikari/jobSync/internal/google/gmail"
	"github.com/saugatadhikari/jobSync/internal/google/sheets"
	"github.com/saugatadhikari/jobSync/internal/storage"
	"github.com/saugatadhikari/jobSync/internal/syncer"
)

func runSync(args []string) error {
	dryRun := false
	limit := int64(0)
	var since time.Time
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run":
			dryRun = true
		case "--limit":
			if i+1 >= len(args) {
				return fmt.Errorf("--limit requires a number")
			}
			i++
			n, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil || n < 1 {
				return fmt.Errorf("invalid --limit %q", args[i])
			}
			limit = n
		case "--since":
			if i+1 >= len(args) {
				return fmt.Errorf("--since requires a date (YYYY-MM-DD)")
			}
			i++
			t, err := time.Parse("2006-01-02", args[i])
			if err != nil {
				return fmt.Errorf("invalid --since %q (use YYYY-MM-DD)", args[i])
			}
			since = t.UTC()
		case "--help", "-h":
			fmt.Println(`Usage:
  jobsync sync [--limit N] [--dry-run] [--since YYYY-MM-DD]

  --since  rescan mail from this date, even if an earlier sync skipped it`)
			return nil
		default:
			return fmt.Errorf("unknown sync flag %q", args[i])
		}
	}
	if limit == 0 {
		limit = 15
	}
	return runFullSync(limit, dryRun, since)
}

func runFullSync(limit int64, dryRun bool, since time.Time) error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if !cfg.HasGeminiKey() {
		return fmt.Errorf("no Gemini API key — run: jobsync init")
	}
	if cfg.SpreadsheetID == "" {
		return fmt.Errorf("no spreadsheet configured — run: jobsync init")
	}

	httpClient, err := auth.EnsureScopes(ctx, cfg, auth.RequiredScopes, auth.CurrentScopesVersion)
	if err != nil {
		return err
	}

	dbPath, err := config.DBPath()
	if err != nil {
		return err
	}
	database, err := storage.Open(dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = storage.Close(database) }()

	gclient, err := gmail.NewClient(ctx, httpClient)
	if err != nil {
		return err
	}
	geminiClient, err := gemini.NewClient(gemini.Options{
		APIKey: cfg.GeminiAPIKey,
		Model:  cfg.GeminiModel,
	})
	if err != nil {
		return err
	}
	sheetsClient, err := sheets.NewClient(ctx, httpClient, cfg.SpreadsheetID, cfg.SheetName)
	if err != nil {
		return err
	}

	if !since.IsZero() && !dryRun {
		n, err := database.ForgetIgnoredEmails(ctx, since)
		if err != nil {
			return err
		}
		fmt.Printf("Rescanning from %s (cleared %d skipped/errored records)\n", since.Format("2006-01-02"), n)
	}

	mode := "full"
	if dryRun {
		mode = "dry-run (no writes)"
	}
	fmt.Printf("Sync starting (%s, max %d Gemini calls)...\n", mode, limit)
	fmt.Printf("Sheet: %s\n\n", sheets.SpreadsheetURL(cfg.SpreadsheetID))

	runner := &syncer.Runner{
		Gmail:  gclient,
		Gemini: geminiClient,
		Sheets: sheetsClient,
		DB:     database,
		Log: func(format string, args ...any) {
			fmt.Printf("• "+format+"\n", args...)
		},
	}

	res, err := runner.Run(ctx, syncer.Options{
		Limit:  limit,
		DryRun: dryRun,
		Since:  since,
	})
	if res != nil {
		fmt.Println()
		fmt.Printf("Status:            %s\n", res.Status)
		fmt.Printf("Emails seen:       %d\n", res.EmailsSeen)
		fmt.Printf("Created:           %d\n", res.EmailsCreated)
		fmt.Printf("Updated:           %d\n", res.EmailsUpdated)
		fmt.Printf("Ignored:           %d\n", res.EmailsIgnored)
		fmt.Printf("Already processed: %d\n", res.EmailsSkippedProcessed)
		fmt.Printf("Gemini calls:      %d\n", res.GeminiCalls)
		fmt.Printf("Errors:            %d\n", res.Errors)
		if res.QuotaExhausted {
			fmt.Println("Quota:             exhausted — remaining mail waits for next sync")
		}
	}
	if err != nil {
		return err
	}
	if dryRun {
		fmt.Println("\nDry-run complete — nothing was written.")
	} else {
		fmt.Println("\nSync complete.")
	}
	return nil
}
