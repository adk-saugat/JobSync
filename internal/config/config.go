package config

import (
	"bufio"
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
	// CloudSyncEnabled means daily sync runs on Cloud Run.
	CloudSyncEnabled bool `json:"cloud_sync_enabled,omitempty"`
	// CloudAccountID is the Neon tenant id used by cloud push (usually derived from Gmail).
	CloudAccountID string `json:"cloud_account_id,omitempty"`
	// CloudServerURL is the hosted JobSync API used for cloud push.
	CloudServerURL string `json:"cloud_server_url,omitempty"`
}

// DefaultCloudServerURL may be set at link time for release binaries.
var DefaultCloudServerURL = ""

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

func LoadDotEnv() {
	for _, path := range dotEnvPaths() {
		if err := loadDotEnvFile(path); err == nil {
			return
		}
	}
}

func dotEnvPaths() []string {
	var paths []string
	if override := os.Getenv("JOBSYNC_CONFIG_DIR"); override != "" {
		paths = append(paths, filepath.Join(override, ".env"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", AppName, ".env"))
	}
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, ".env"))
	}
	return paths
}

func loadDotEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		value = strings.Trim(value, `"'`)
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
	return scanner.Err()
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

func filePath(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

func DBPath() (string, error)           { return filePath(DBFile) }
func TokenPath() (string, error)        { return filePath(TokenFile) }
func ClientSecretPath() (string, error) { return filePath(ClientSecretFile) }
func ConfigPath() (string, error)       { return filePath(ConfigFile) }

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

// UsesCloudSync reports whether daily sync is delegated to Cloud Run.
func (c *Config) UsesCloudSync() bool {
	return c != nil && c.CloudSyncEnabled
}
