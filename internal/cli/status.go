package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/saugatadhikari/jobSync/internal/config"
	"github.com/saugatadhikari/jobSync/internal/domain"
	"github.com/saugatadhikari/jobSync/internal/google/sheets"
	"github.com/saugatadhikari/jobSync/internal/storage"
)

func runStatus(args []string) error {
	_ = args

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	dir, err := config.Dir()
	if err != nil {
		return err
	}
	fmt.Printf("config dir: %s\n", dir)

	if cfg.SpreadsheetID == "" {
		fmt.Println("spreadsheet: (not set — run jobsync init)")
	} else {
		fmt.Printf("spreadsheet: %s\n", sheets.SpreadsheetURL(cfg.SpreadsheetID))
	}

	tokenPath, err := config.TokenPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(tokenPath); err == nil {
		fmt.Println("google auth: ok")
	} else {
		fmt.Println("google auth: (run jobsync init)")
	}

	if cfg.HasGeminiKey() {
		fmt.Printf("gemini:      ok (model %s)\n", cfg.GeminiModel)
	} else {
		fmt.Println("gemini:      (run jobsync init)")
	}

	if cfg.UsesCloudSync() {
		fmt.Println("daily sync:  cloud (server)")
		if cfg.CloudAccountID != "" {
			fmt.Printf("cloud id:    %s\n", cfg.CloudAccountID)
		}
	} else {
		fmt.Println("daily sync:  (run jobsync cloud push)")
	}

	dbPath, err := config.DBPath()
	if err != nil {
		return err
	}
	database, err := storage.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = storage.Close(database) }()

	run, err := database.GetLastSyncRun(context.Background())
	if err != nil {
		return err
	}
	fmt.Printf("last local sync: %s\n", formatLastSync(run))
	return nil
}

func formatLastSync(run *domain.SyncRun) string {
	if run == nil || run.FinishedAt == nil {
		return "(none — cloud sync history is on the server)"
	}
	when := run.FinishedAt.Local().Format("2006-01-02 15:04 MST")
	summary := fmt.Sprintf("%s — %s, %d updated", when, run.Status, run.EmailsUpdated)
	return summary
}
