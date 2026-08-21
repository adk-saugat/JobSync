package auth

import "testing"

func TestLoadOAuthConfigUsesEmbeddedClient(t *testing.T) {
	t.Setenv("JOBSYNC_CLIENT_SECRET_FILE", "")
	t.Setenv("JOBSYNC_CONFIG_DIR", t.TempDir())

	cfg, err := LoadOAuthConfig(RequiredScopes)
	if err != nil {
		t.Fatalf("LoadOAuthConfig: %v", err)
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		t.Fatal("expected embedded client_id and client_secret")
	}
	if len(cfg.Scopes) != len(RequiredScopes) {
		t.Fatalf("scopes = %v, want %v", cfg.Scopes, RequiredScopes)
	}
}
