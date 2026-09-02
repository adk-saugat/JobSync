package cli

import (
	"fmt"
	"strings"
)

// Run dispatches CLI commands.
func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return fmt.Errorf("missing command")
	}

	switch strings.ToLower(args[0]) {
	case "init":
		return runInit(args[1:])
	case "sync":
		return runSync(args[1:])
	case "status":
		return runStatus(args[1:])
	case "cloud":
		return runCloud(args[1:])
	case "version", "-v", "--version":
		return runVersion(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage() {
	fmt.Println(`JobSync — sync job emails from Gmail to Google Sheets

Usage:
  jobsync <command>

Commands:
  init      Google sign-in, Sheet setup, Gemini key
  sync      Full sync (--dry-run, --limit, --since)
  status    Show spreadsheet, auth, Gemini, cloud sync
  cloud     Register for hosted daily sync (cloud push)
  version   Show version
  help      Show this help`)
}
