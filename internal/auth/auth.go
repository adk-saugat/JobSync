package auth

import (
	"context"
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
	"google.golang.org/api/sheets/v4"

	"github.com/saugatadhikari/jobSync/internal/config"
)

// SheetsScopes are enough for Phase 2 (tracker read/write).
var SheetsScopes = []string{
	sheets.SpreadsheetsScope,
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

// LoadOAuthConfig loads a Desktop OAuth client from client_secret.json.
func LoadOAuthConfig(scopes []string) (*oauth2.Config, error) {
	path, err := config.ClientSecretPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: place your Desktop OAuth JSON at %s (see docs/GOOGLE_SETUP.md)", ErrMissingClientSecret, path)
		}
		return nil, err
	}

	var raw installedCredentials
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse client secret: %w", err)
	}

	clientID := raw.Installed.ClientID
	clientSecret := raw.Installed.ClientSecret
	if clientID == "" {
		clientID = raw.Web.ClientID
		clientSecret = raw.Web.ClientSecret
	}
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("client_secret.json missing client_id/client_secret")
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       scopes,
		// Redirect is set per login to the local callback server.
	}, nil
}

// TokenFromFile loads a saved OAuth token.
func TokenFromFile() (*oauth2.Token, error) {
	path, err := config.TokenPath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	if err := json.NewDecoder(f).Decode(tok); err != nil {
		return nil, err
	}
	return tok, nil
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
	// Persist refreshed tokens.
	returning := &persistTokenSource{
		base: oauth2.ReuseTokenSource(tok, tokenSource),
		tok:  tok,
	}
	return oauth2.NewClient(ctx, returning), nil
}

type persistTokenSource struct {
	base oauth2.TokenSource
	tok  *oauth2.Token
}

func (p *persistTokenSource) Token() (*oauth2.Token, error) {
	tok, err := p.base.Token()
	if err != nil {
		return nil, err
	}
	if tok.AccessToken != p.tok.AccessToken || tok.RefreshToken != p.tok.RefreshToken {
		_ = SaveToken(tok)
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
