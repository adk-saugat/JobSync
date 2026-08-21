package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/saugatadhikari/jobSync/internal/auth"
	"github.com/saugatadhikari/jobSync/internal/config"
	"github.com/saugatadhikari/jobSync/internal/gmail"
	"github.com/saugatadhikari/jobSync/internal/llm"
)

func runSync(args []string) error {
	emailsOnly := false
	extract := false
	limit := int64(10)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--emails-only", "-e":
			emailsOnly = true
		case "--extract":
			extract = true
			if limit == 10 {
				limit = 2 // default small for free-tier safety
			}
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
  jobsync sync --emails-only [--limit N]   List job-like Gmail messages (no Gemini)
  jobsync sync --extract [--limit N]       Extract fields with Gemini (default limit 2; no Sheet writes)
  jobsync sync                             Full sync (Phase 5 — not ready yet)`)
			return nil
		default:
			return fmt.Errorf("unknown sync flag %q (try --emails-only or --extract)", args[i])
		}
	}

	if extract {
		return extractJobEmails(limit)
	}
	if emailsOnly {
		return listJobEmails(limit)
	}

	fmt.Println("Full sync is not ready yet (Phase 5).")
	fmt.Println("For now:")
	fmt.Println("  jobsync sync --emails-only")
	fmt.Println("  jobsync sync --extract --limit 2")
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
		fmt.Println("No candidate emails found. Try widening the query later or wait for new mail.")
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

	fmt.Printf("Listed %d job-like emails (%d filtered out by local pre-filter).\n", shown, skipped)
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
	llmClient, err := llm.NewClient(llm.Options{
		APIKey: cfg.GeminiAPIKey,
		Model:  cfg.GeminiModel,
	})
	if err != nil {
		return err
	}

	// Fetch a slightly larger Gmail pool, then stop after `limit` Gemini calls.
	searchLimit := limit * 3
	if searchLimit < 10 {
		searchLimit = 10
	}

	fmt.Printf("Extracting with Gemini model %s (max %d emails, no Sheet writes)...\n\n", cfg.GeminiModel, limit)

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

		ext, err := llmClient.Extract(ctx, llm.EmailInput{
			Subject: msg.Subject,
			From:    msg.From,
			Date:    msg.Date,
			Body:    msg.Body,
		})
		geminiCalls++
		if err != nil {
			if errors.Is(err, llm.ErrQuotaExceeded) {
				fmt.Printf("Gemini quota hit after %d calls — stop for today.\n", geminiCalls)
				return err
			}
			fmt.Printf("Extract error: %v\n\n", err)
			continue
		}

		pretty, _ := json.MarshalIndent(ext, "  ", "  ")
		fmt.Printf("Result:\n  %s\n", string(pretty))
		if !ext.IsJobRelated {
			fmt.Println("→ ignored (not job related)")
		} else if ext.Confidence < llmClient.MinConfidence() {
			fmt.Printf("→ low confidence (%.2f < %.2f)\n", ext.Confidence, llmClient.MinConfidence())
		} else {
			fmt.Printf("→ would upsert: %s / %s [%s]\n", ext.Company, ext.Position, ext.Status)
		}
		fmt.Println()

		// Light pacing for free tier.
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Printf("Done. Gemini calls: %d, pre-filter skipped: %d\n", geminiCalls, skippedPre)
	fmt.Println("Sheet writes come in Phase 5 (full sync).")
	return nil
}
