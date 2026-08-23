package auth

import "testing"

func TestLoadOAuthConfigFromEnv(t *testing.T) {
	t.Setenv("JOBSYNC_CLIENT_SECRET_FILE", "")
	t.Setenv("JOBSYNC_CONFIG_DIR", t.TempDir())
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "test-client-secret")

	cfg, err := LoadOAuthConfig(RequiredScopes)
	if err != nil {
		t.Fatalf("LoadOAuthConfig: %v", err)
	}
	if cfg.ClientID != "test-client-id" || cfg.ClientSecret != "test-client-secret" {
		t.Fatalf("got %q / %q", cfg.ClientID, cfg.ClientSecret)
	}
	if len(cfg.Scopes) != len(RequiredScopes) {
		t.Fatalf("scopes = %v, want %v", cfg.Scopes, RequiredScopes)
	}
}

func TestLoadOAuthConfigMissing(t *testing.T) {
	t.Setenv("JOBSYNC_CLIENT_SECRET_FILE", "")
	t.Setenv("JOBSYNC_CONFIG_DIR", t.TempDir())
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "")
	EmbeddedClientID = ""
	EmbeddedClientSecret = ""

	_, err := LoadOAuthConfig(RequiredScopes)
	if err == nil {
		t.Fatal("expected error when no OAuth client configured")
	}
}
