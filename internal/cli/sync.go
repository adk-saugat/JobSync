package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	emailsOnly := false
	extract := false
	dryRun := false
	limit := int64(0) // 0 = command-specific default
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--emails-only", "-e":
			emailsOnly = true
		case "--extract":
			extract = true
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
		case "--help", "-h":
			fmt.Println(`Usage:
  jobsync sync [--limit N] [--dry-run]     Full sync: Gmail → Gemini → SQLite → Sheet
  jobsync sync --emails-only [--limit N]   List status-update emails (no Gemini)
  jobsync sync --extract [--limit N]       Extract with Gemini only (no writes)`)
			return nil
		default:
			return fmt.Errorf("unknown sync flag %q", args[i])
		}
	}

	if extract {
		if limit == 0 {
			limit = 2
		}
		return extractJobEmails(limit)
	}
	if emailsOnly {
		if limit == 0 {
			limit = 10
		}
		return listJobEmails(limit)
	}
	if limit == 0 {
		limit = 15
	}
	return runFullSync(limit, dryRun)
}

func runFullSync(limit int64, dryRun bool) error {
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
	})
	if res != nil {
		fmt.Println()
		fmt.Printf("Status:            %s\n", res.Status)
		fmt.Printf("Emails seen:       %d\n", res.EmailsSeen)
		fmt.Printf("Created:           %d\n", res.EmailsCreated)
		fmt.Printf("Updated:           %d\n", res.EmailsUpdated)
		fmt.Printf("Ignored:           %d\n", res.EmailsIgnored)
		fmt.Printf("Already processed: %d\n", res.EmailsSkippedProcessed)
		fmt.Printf("Pre-filter skip:   %d\n", res.GeminiSkippedPrefilter)
		fmt.Printf("Gemini calls:      %d\n", res.GeminiCalls)
		fmt.Printf("Errors:            %d\n", res.Errors)
		if res.QuotaExhausted {
			fmt.Println("Quota:             exhausted — remaining mail waits for next sync")
		}
		if res.Watermark != "" {
			fmt.Printf("Watermark:         %s\n", res.Watermark)
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

func listJobEmails(limit int64) error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	httpClient, err := auth.EnsureScopes(ctx, cfg, auth.RequiredScopes, auth.CurrentScopesVersion)
	if err != nil {
		return err
	}

	client, err := gmail.NewClient(ctx, httpClient)
	if err != nil {
		return err
	}

	fmt.Printf("Searching Gmail (limit %d, no Gemini)...\n", limit)
	fmt.Printf("Query: %s\n\n", gmail.DefaultQuery)

	metas, err := client.Search(ctx, gmail.DefaultQuery, limit, time.Time{})
	if err != nil {
		return err
	}
	if len(metas) == 0 {
		fmt.Println("No candidate emails found.")
		return nil
	}

	shown := 0
	skipped := 0
	for _, meta := range metas {
		msg, err := client.GetMessage(ctx, meta.ID)
		if err != nil {
			return err
		}
		if !gmail.LooksLikeStatusUpdate(msg.Subject, msg.From) {
			skipped++
			continue
		}
		shown++
		bodyPreview := msg.Body
		if len(bodyPreview) > 160 {
			bodyPreview = bodyPreview[:160] + "…"
		}
		bodyPreview = strings.ReplaceAll(bodyPreview, "\n", " ")

		fmt.Printf("%d) %s\n", shown, msg.Date.Local().Format("2006-01-02 15:04"))
		fmt.Printf("   From:    %s\n", msg.From)
		fmt.Printf("   Subject: %s\n", msg.Subject)
		fmt.Printf("   ID:      %s\n", msg.ID)
		fmt.Printf("   Preview: %s\n\n", bodyPreview)
	}

	fmt.Printf("Listed %d status-update emails (%d filtered out locally).\n", shown, skipped)
	return nil
}

func extractJobEmails(limit int64) error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if !cfg.HasGeminiKey() {
		return fmt.Errorf("no Gemini API key — run: jobsync init")
	}

	httpClient, err := auth.EnsureScopes(ctx, cfg, auth.RequiredScopes, auth.CurrentScopesVersion)
	if err != nil {
		return err
	}
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

	searchLimit := limit * 3
	if searchLimit < 10 {
		searchLimit = 10
	}

	fmt.Printf("Extracting with Gemini model %s (max %d emails, no writes)...\n\n", cfg.GeminiModel, limit)

	metas, err := gclient.Search(ctx, gmail.DefaultQuery, searchLimit, time.Time{})
	if err != nil {
		return err
	}

	geminiCalls := 0
	skippedPre := 0
	for _, meta := range metas {
		if int64(geminiCalls) >= limit {
			break
		}
		msg, err := gclient.GetMessage(ctx, meta.ID)
		if err != nil {
			return err
		}
		if !gmail.LooksLikeStatusUpdate(msg.Subject, msg.From) {
			skippedPre++
			continue
		}

		fmt.Printf("Email: %s\n", msg.Subject)
		fmt.Printf("From:  %s\n", msg.From)

		ext, err := geminiClient.Extract(ctx, gemini.EmailInput{
			Subject: msg.Subject,
			From:    msg.From,
			Date:    msg.Date,
			Body:    msg.Body,
		})
		geminiCalls++
		if err != nil {
			if errors.Is(err, gemini.ErrQuotaExceeded) {
				fmt.Printf("Gemini quota hit after %d calls — stop for today.\n", geminiCalls)
				return err
			}
			fmt.Printf("Extract error: %v\n\n", err)
			continue
		}

		pretty, _ := json.MarshalIndent(ext, "  ", "  ")
		fmt.Printf("Result:\n  %s\n", string(pretty))
		if !ext.IsJobRelated {
			fmt.Println("→ ignored (not a status update)")
		} else if ext.Confidence < geminiClient.MinConfidence() {
			fmt.Printf("→ low confidence (%.2f < %.2f)\n", ext.Confidence, geminiClient.MinConfidence())
		} else {
			fmt.Printf("→ would upsert: %s / %s [%s]\n", ext.Company, ext.Position, ext.Status)
		}
		fmt.Println()
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Printf("Done. Gemini calls: %d, pre-filter skipped: %d\n", geminiCalls, skippedPre)
	return nil
}
