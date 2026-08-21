package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/saugatadhikari/jobSync/internal/auth"
	"github.com/saugatadhikari/jobSync/internal/config"
	"github.com/saugatadhikari/jobSync/internal/gmail"
	"github.com/saugatadhikari/jobSync/internal/models"
	"github.com/saugatadhikari/jobSync/internal/sheets"
)

func runInit(args []string) error {
	_ = args
	ctx := context.Background()

	dir, err := config.EnsureDir()
	if err != nil {
		return err
	}
	fmt.Printf("config dir: %s\n", dir)

	secretPath, err := config.ClientSecretPath()
	if err != nil {
		return err
	}
	fmt.Printf("Looking for Google OAuth client at:\n  %s\n", secretPath)
	fmt.Println("(See docs/GOOGLE_SETUP.md if you have not created this file yet.)")
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
		fmt.Printf("Using existing spreadsheet: %s\n", sheets.SpreadsheetURL(cfg.SpreadsheetID))
		client, err := sheets.NewClient(ctx, httpClient, cfg.SpreadsheetID, cfg.SheetName)
		if err != nil {
			return err
		}
		fmt.Println("Refreshing sheet layout + formatting...")
		if err := client.SetupSheet(ctx); err != nil {
			return err
		}
	}

	client, err := sheets.NewClient(ctx, httpClient, cfg.SpreadsheetID, cfg.SheetName)
	if err != nil {
		return err
	}

	rowID := uuid.NewString()
	testRow := sheets.Row{
		RowID:     rowID,
		Company:   "Phase2 Test Co",
		Position:  "Software Engineer",
		Status:    models.StatusApplied,
		AppliedAt: "2026-08-01",
		Notes:     "Created by jobsync init",
	}
	if err := client.AppendRow(ctx, testRow); err != nil {
		return fmt.Errorf("append test row: %w", err)
	}
	fmt.Println("Appended test row (Row ID stored, column hidden)")

	testRow.Status = models.StatusInterview
	testRow.InterviewAt = "2026-08-20"
	testRow.Notes = "Updated by jobsync init — status should be green"
	if err := client.UpdateRowByID(ctx, testRow); err != nil {
		return fmt.Errorf("update test row: %w", err)
	}
	fmt.Println("Updated test row by Row ID → interview")

	gclient, err := gmail.NewClient(ctx, httpClient)
	if err != nil {
		return err
	}
	msgs, err := gclient.Search(ctx, gmail.DefaultQuery, 5, time.Time{})
	if err != nil {
		return fmt.Errorf("gmail search (enable Gmail API in Google Cloud if this fails): %w", err)
	}
	fmt.Printf("Gmail search: ok (%d recent candidate emails)\n", len(msgs))

	fmt.Println()
	fmt.Println("Init complete.")
	fmt.Printf("Sheet: %s\n", sheets.SpreadsheetURL(cfg.SpreadsheetID))
	fmt.Println("Next: ./bin/jobsync sync --emails-only")
	return nil
}
