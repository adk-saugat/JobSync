package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/saugatadhikari/jobSync/internal/auth"
	"github.com/saugatadhikari/jobSync/internal/config"
	"github.com/saugatadhikari/jobSync/internal/gmail"
)

func runSync(args []string) error {
	emailsOnly := false
	limit := int64(10)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--emails-only", "-e":
			emailsOnly = true
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
  jobsync sync                             Full sync (Phase 5 — not ready yet)`)
			return nil
		default:
			return fmt.Errorf("unknown sync flag %q (try --emails-only)", args[i])
		}
	}

	if !emailsOnly {
		fmt.Println("Full sync is not ready yet (Phase 5).")
		fmt.Println("For now, list emails with:")
		fmt.Println("  jobsync sync --emails-only")
		return nil
	}

	return listJobEmails(limit)
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
		if !gmail.LooksJobRelated(msg.Subject, msg.From) {
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
	fmt.Println("Gemini extraction comes in Phase 4.")
	return nil
}
