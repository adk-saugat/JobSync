package cli

import (
	"fmt"

	"github.com/saugatadhikari/jobSync/internal/config"
	"github.com/saugatadhikari/jobSync/internal/db"
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

	fmt.Println("database:   ok (migrations applied)")
	fmt.Println("next run:   (not scheduled yet — Phase 6)")
	fmt.Println("last sync:  (none yet)")
	return nil
}
