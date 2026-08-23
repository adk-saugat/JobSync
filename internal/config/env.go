package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadDotEnv loads KEY=VALUE pairs from the first .env file found.
// Existing environment variables are not overwritten.
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
