package cli

import (
	"fmt"
	"os"

	"github.com/saugatadhikari/jobSync/internal/config"
	"github.com/saugatadhikari/jobSync/internal/db"
	"github.com/saugatadhikari/jobSync/internal/sheets"
)

func runStatus(args []string) error {
	_ = args

	dir, err := config.Dir()
	if err != nil {
		return err
	}
	dbPath, err := config.DBPath()
	if err != nil {
		return err
	}

	fmt.Printf("config dir: %s\n", dir)
	fmt.Printf("database:   %s\n", dbPath)

	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = db.Close(database) }()
	fmt.Println("database:   ok")

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.SpreadsheetID == "" {
		fmt.Println("spreadsheet: (not set — run jobsync init)")
	} else {
		fmt.Printf("spreadsheet: %s\n", sheets.SpreadsheetURL(cfg.SpreadsheetID))
		fmt.Printf("sheet tab:   %s\n", cfg.SheetName)
	}

	tokenPath, err := config.TokenPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(tokenPath); err == nil {
		fmt.Println("google auth: token present")
	} else {
		fmt.Println("google auth: (not signed in — run jobsync init)")
	}

	if cfg.SyncHour != nil && cfg.SyncMinute != nil {
		fmt.Printf("next run:    daily at %02d:%02d local\n", *cfg.SyncHour, *cfg.SyncMinute)
	} else {
		fmt.Println("next run:    (not scheduled yet — Phase 6)")
	}
	fmt.Println("last sync:   (none yet)")
	return nil
}
