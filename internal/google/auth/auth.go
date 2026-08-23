package auth

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/sheets/v4"

	"github.com/saugatadhikari/jobSync/internal/config"
)

//go:embed oauth_client.json
var embeddedOAuthClientJSON []byte

// CurrentScopesVersion bumps when required OAuth scopes change (forces re-login).
const CurrentScopesVersion = 2 // Sheets + Gmail readonly

// RequiredScopes are used by init/sync for Sheets + Gmail.
var RequiredScopes = []string{
	sheets.SpreadsheetsScope,
	gmail.GmailReadonlyScope,
}

// ErrMissingClientSecret means the user has not installed Google OAuth credentials yet.
var ErrMissingClientSecret = errors.New("missing Google OAuth client secret")

type installedCredentials struct {
	Installed struct {
		ClientID     string   `json:"client_id"`
		ClientSecret string   `json:"client_secret"`
		RedirectURIs []string `json:"redirect_uris"`
	} `json:"installed"`
	Web struct {
		ClientID     string   `json:"client_id"`
		ClientSecret string   `json:"client_secret"`
		RedirectURIs []string `json:"redirect_uris"`
	} `json:"web"`
}

// LoadOAuthConfig loads the Desktop OAuth client.
// Order: JOBSYNC_CLIENT_SECRET_FILE → ~/.config/jobsync/client_secret.json → embedded JobSync client.
func LoadOAuthConfig(scopes []string) (*oauth2.Config, error) {
	data, err := readOAuthClientJSON()
	if err != nil {
		return nil, err
	}

	var raw installedCredentials
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse OAuth client JSON: %w", err)
	}

	clientID := raw.Installed.ClientID
	clientSecret := raw.Installed.ClientSecret
	if clientID == "" {
		clientID = raw.Web.ClientID
		clientSecret = raw.Web.ClientSecret
	}
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("OAuth client JSON missing client_id/client_secret")
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       scopes,
		// Redirect is set per login to the local callback server.
	}, nil
}

func readOAuthClientJSON() ([]byte, error) {
	if path := os.Getenv("JOBSYNC_CLIENT_SECRET_FILE"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read JOBSYNC_CLIENT_SECRET_FILE: %w", err)
		}
		return data, nil
	}

	path, err := config.ClientSecretPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err == nil {
		return data, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}

	if len(embeddedOAuthClientJSON) == 0 {
		return nil, fmt.Errorf("%w: no embedded OAuth client and none at %s (see README.md)", ErrMissingClientSecret, path)
	}
	return embeddedOAuthClientJSON, nil
}

// TokenFromFile loads a saved OAuth token.
func TokenFromFile() (*oauth2.Token, error) {
	path, err := config.TokenPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return TokenFromJSON(data)
}

// TokenFromJSON parses a saved OAuth token.
func TokenFromJSON(data []byte) (*oauth2.Token, error) {
	tok := &oauth2.Token{}
	if err := json.Unmarshal(data, tok); err != nil {
		return nil, fmt.Errorf("parse oauth token: %w", err)
	}
	if tok.AccessToken == "" && tok.RefreshToken == "" {
		return nil, fmt.Errorf("oauth token json is empty")
	}
	return tok, nil
}

// MarshalTokenJSON encodes a token for persistence.
func MarshalTokenJSON(tok *oauth2.Token) ([]byte, error) {
	if tok == nil {
		return nil, fmt.Errorf("token is nil")
	}
	return json.Marshal(tok)
}

// SaveToken writes the OAuth token to disk.
func SaveToken(tok *oauth2.Token) error {
	if _, err := config.EnsureDir(); err != nil {
		return err
	}
	path, err := config.TokenPath()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(tok)
}

// Login runs a localhost OAuth flow and saves the token.
func Login(ctx context.Context, scopes []string) (*oauth2.Token, error) {
	cfg, err := LoadOAuthConfig(scopes)
	if err != nil {
		return nil, err
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen for oauth callback: %w", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	cfg.RedirectURL = fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	state := fmt.Sprintf("jobsync-%d", time.Now().UnixNano())
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errCh <- fmt.Errorf("invalid oauth state")
			http.Error(w, "invalid state", http.StatusBadRequest)
			return
		}
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			errCh <- fmt.Errorf("oauth error: %s", errMsg)
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("missing oauth code")
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, "JobSync authorization OK. You can close this tab and return to the terminal.")
		codeCh <- code
	})

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	authURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Println("Opening browser for Google sign-in...")
	fmt.Println(authURL)
	_ = openBrowser(authURL)

	var code string
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case code = <-codeCh:
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("oauth timed out waiting for browser login")
	}

	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	if err := SaveToken(tok); err != nil {
		return nil, err
	}
	return tok, nil
}

// ClearToken removes the saved OAuth token (forces the next login).
func ClearToken() error {
	path, err := config.TokenPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// HTTPClient returns an authenticated HTTP client, refreshing/saving tokens as needed.
// If no token exists, it runs Login.
func HTTPClient(ctx context.Context, scopes []string) (*http.Client, error) {
	cfg, err := LoadOAuthConfig(scopes)
	if err != nil {
		return nil, err
	}

	tok, err := TokenFromFile()
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		tok, err = Login(ctx, scopes)
		if err != nil {
			return nil, err
		}
	}

	tokenSource := cfg.TokenSource(ctx, tok)
	returning := &persistTokenSource{
		base: oauth2.ReuseTokenSource(tok, tokenSource),
		tok:  tok,
	}
	return oauth2.NewClient(ctx, returning), nil
}

// HTTPClientFromStoredToken builds an HTTP client from JSON token bytes.
// onSave is called when the token is refreshed (e.g. write back to Neon).
func HTTPClientFromStoredToken(ctx context.Context, tokenJSON string, onSave func([]byte) error) (*http.Client, error) {
	tok, err := TokenFromJSON([]byte(tokenJSON))
	if err != nil {
		return nil, err
	}
	oauthCfg, err := LoadOAuthConfig(RequiredScopes)
	if err != nil {
		return nil, err
	}
	tokenSource := oauthCfg.TokenSource(ctx, tok)
	returning := &persistTokenSource{
		base: oauth2.ReuseTokenSource(tok, tokenSource),
		tok:  tok,
		save: onSave,
	}
	return oauth2.NewClient(ctx, returning), nil
}

// EnsureScopes returns an HTTP client with the required scopes.
// If the saved auth version is older, it clears the token and re-runs Login.
func EnsureScopes(ctx context.Context, cfg *config.Config, scopes []string, scopesVersion int) (*http.Client, error) {
	if cfg.AuthScopesVersion < scopesVersion {
		fmt.Println("Google permissions updated — please sign in again (Sheets + Gmail)...")
		if err := ClearToken(); err != nil {
			return nil, err
		}
		if _, err := Login(ctx, scopes); err != nil {
			return nil, err
		}
		cfg.AuthScopesVersion = scopesVersion
		if err := config.Save(cfg); err != nil {
			return nil, err
		}
	}
	return HTTPClient(ctx, scopes)
}

type persistTokenSource struct {
	base oauth2.TokenSource
	tok  *oauth2.Token
	save func([]byte) error
}

func (p *persistTokenSource) Token() (*oauth2.Token, error) {
	tok, err := p.base.Token()
	if err != nil {
		return nil, err
	}
	if tok.AccessToken != p.tok.AccessToken || tok.RefreshToken != p.tok.RefreshToken {
		if p.save != nil {
			if data, err := MarshalTokenJSON(tok); err == nil {
				_ = p.save(data)
			}
		} else {
			_ = SaveToken(tok)
		}
		p.tok = tok
	}
	return tok, nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
