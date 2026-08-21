package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	AppName            = "jobsync"
	ConfigFile         = "config.json"
	DBFile             = "jobsync.db"
	TokenFile          = "token.json"
	ClientSecretFile   = "client_secret.json"
	DefaultSheetName   = "Applications"
	DefaultGeminiModel = "gemini-3.6-flash"
)

// Config is persisted user settings under the config directory.
type Config struct {
	SpreadsheetID string `json:"spreadsheet_id,omitempty"`
	SheetName     string `json:"sheet_name,omitempty"`
	GeminiAPIKey  string `json:"gemini_api_key,omitempty"`
	GeminiModel   string `json:"gemini_model,omitempty"`
	// AuthScopesVersion tracks which Google OAuth scopes were granted.
	AuthScopesVersion int `json:"auth_scopes_version,omitempty"`
	// SyncHour and SyncMinute are set in Phase 6 (random evening time).
	SyncHour   *int `json:"sync_hour,omitempty"`
	SyncMinute *int `json:"sync_minute,omitempty"`
}

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

// TokenPath returns the OAuth token path.
func TokenPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, TokenFile), nil
}

// ClientSecretPath returns the Google OAuth client secret JSON path.
func ClientSecretPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ClientSecretFile), nil
}

// ConfigPath returns the path to config.json.
func ConfigPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ConfigFile), nil
}

// Load reads config.json, or returns empty config if missing.
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{SheetName: DefaultSheetName, GeminiModel: DefaultGeminiModel}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.SheetName == "" {
		cfg.SheetName = DefaultSheetName
	}
	if cfg.GeminiModel == "" || isRetiredGeminiModel(cfg.GeminiModel) {
		cfg.GeminiModel = DefaultGeminiModel
	}
	return &cfg, nil
}

func isRetiredGeminiModel(model string) bool {
	switch strings.TrimSpace(model) {
	case "gemini-2.0-flash", "gemini-2.0-flash-001", "gemini-1.5-flash", "gemini-1.5-flash-latest":
		return true
	default:
		return false
	}
}

// Save writes config.json with restrictive permissions.
func Save(cfg *Config) error {
	if _, err := EnsureDir(); err != nil {
		return err
	}
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if cfg.SheetName == "" {
		cfg.SheetName = DefaultSheetName
	}
	if cfg.GeminiModel == "" || isRetiredGeminiModel(cfg.GeminiModel) {
		cfg.GeminiModel = DefaultGeminiModel
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// HasGeminiKey reports whether a Gemini API key is configured.
func (c *Config) HasGeminiKey() bool {
	return c != nil && strings.TrimSpace(c.GeminiAPIKey) != ""
}
