package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvSetsUnsetVars(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("DATABASE_URL=postgres://test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JOBSYNC_CONFIG_DIR", dir)

	LoadDotEnv()
	if got := os.Getenv("DATABASE_URL"); got != "postgres://test" {
		t.Fatalf("DATABASE_URL = %q", got)
	}
}

func TestLoadDotEnvDoesNotOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("DATABASE_URL=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JOBSYNC_CONFIG_DIR", dir)
	t.Setenv("DATABASE_URL", "already-set")

	LoadDotEnv()
	if got := os.Getenv("DATABASE_URL"); got != "already-set" {
		t.Fatalf("DATABASE_URL = %q, want already-set", got)
	}
}
