package config

import (
	"os"
	"path/filepath"
)

const (
	AppName    = "jobsync"
	ConfigFile = "config.yaml"
	DBFile     = "jobsync.db"
	TokenFile  = "token.json"
)

// Dir returns the JobSync config directory (~/.config/jobsync).
func Dir() (string, error) {
	if override := os.Getenv("JOBSYNC_CONFIG_DIR"); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", AppName), nil
}

// EnsureDir creates the config directory if needed.
func EnsureDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// DBPath returns the default SQLite database path.
func DBPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, DBFile), nil
}
