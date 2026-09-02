package server

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/saugatadhikari/jobSync/internal/cloud/service"
	"github.com/saugatadhikari/jobSync/internal/google/auth"
)

const (
	setupCookieName = "jobsync_setup"
	setupStateCookie = "jobsync_oauth_state"
	setupCookieTTL  = 30 * time.Minute
)

type setupSession struct {
	TokenJSON string `json:"token_json"`
	Email     string `json:"email"`
	Exp       int64  `json:"exp"`
}

type setupCompleteRequest struct {
	GeminiAPIKey string `json:"gemini_api_key"`
}

func (s *Server) handleSetupOAuthStart(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.oauthConfig(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	state, err := randomState()
	if err != nil {
		http.Error(w, "could not start oauth", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     setupStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   int(setupCookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   requestSecure(r),
		SameSite: http.SameSiteLaxMode,
	})

	authURL := cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) handleSetupOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		http.Redirect(w, r, "/?error="+url.QueryEscape(errMsg), http.StatusFound)
		return
	}

	stateCookie, err := r.Cookie(setupStateCookie)
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		http.Redirect(w, r, "/?error="+url.QueryEscape("invalid oauth state"), http.StatusFound)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: setupStateCookie, Value: "", Path: "/", MaxAge: -1})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/?error="+url.QueryEscape("missing oauth code"), http.StatusFound)
		return
	}

	cfg, err := s.oauthConfig(r)
	if err != nil {
		http.Redirect(w, r, "/?error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}

	ctx := r.Context()
	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		http.Redirect(w, r, "/?error="+url.QueryEscape("oauth exchange failed"), http.StatusFound)
		return
	}
	tokenJSON, err := auth.MarshalTokenJSON(tok)
	if err != nil {
		http.Redirect(w, r, "/?error="+url.QueryEscape("could not save token"), http.StatusFound)
		return
	}
	email, err := auth.GoogleEmailFromTokenJSON(ctx, tokenJSON)
	if err != nil {
		http.Redirect(w, r, "/?error="+url.QueryEscape("could not verify google account"), http.StatusFound)
		return
	}

	sess := setupSession{
		TokenJSON: string(tokenJSON),
		Email:     email,
		Exp:       time.Now().Add(setupCookieTTL).Unix(),
	}
	if err := s.setSetupCookie(w, r, sess); err != nil {
		http.Redirect(w, r, "/?error="+url.QueryEscape("could not create session"), http.StatusFound)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) handleSetupSession(w http.ResponseWriter, r *http.Request) {
	sess, err := s.readSetupCookie(r)
	if err != nil || sess == nil {
		writeJSON(w, http.StatusOK, map[string]any{"signed_in": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"signed_in": true,
		"email":     sess.Email,
	})
}

func (s *Server) handleSetupComplete(w http.ResponseWriter, r *http.Request) {
	sess, err := s.readSetupCookie(r)
	if err != nil || sess == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "sign in with Google first"})
		return
	}

	var req setupCompleteRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	ctx := r.Context()
	out, err := service.CompleteWebSetup(ctx, s.DB, []byte(sess.TokenJSON), req.GeminiAPIKey)
	if err != nil {
		log.Printf("setup complete error: %v", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     setupCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   requestSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, out)
	log.Printf("web setup ok account=%s sheet=%s", out.AccountID, out.SpreadsheetID)
}

func (s *Server) oauthConfig(r *http.Request) (*oauth2.Config, error) {
	cfg, err := auth.LoadOAuthConfig(auth.RequiredScopes)
	if err != nil {
		return nil, err
	}
	cfg.RedirectURL = s.setupRedirectURL(r)
	return cfg, nil
}

func (s *Server) setupRedirectURL(r *http.Request) string {
	if v := strings.TrimSpace(os.Getenv("SETUP_OAUTH_REDIRECT_URL")); v != "" {
		return v
	}
	return strings.TrimRight(publicBaseURL(r), "/") + "/setup/oauth/callback"
}

func publicBaseURL(r *http.Request) string {
	if v := strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	scheme := "https"
	if !requestSecure(r) {
		scheme = "http"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = strings.TrimSpace(strings.Split(proto, ",")[0])
	}
	host := r.Host
	if host == "" {
		host = "localhost:8080"
	}
	return scheme + "://" + host
}

func requestSecure(r *http.Request) bool {
	if proto := r.Header.Get("X-Forwarded-Proto"); strings.EqualFold(strings.TrimSpace(strings.Split(proto, ",")[0]), "https") {
		return true
	}
	return r.TLS != nil
}

func (s *Server) setSetupCookie(w http.ResponseWriter, r *http.Request, sess setupSession) error {
	raw, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	enc, err := seal(s.SyncSecret, raw)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     setupCookieName,
		Value:    enc,
		Path:     "/",
		MaxAge:   int(setupCookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   requestSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (s *Server) readSetupCookie(r *http.Request) (*setupSession, error) {
	c, err := r.Cookie(setupCookieName)
	if err != nil || c.Value == "" {
		return nil, fmt.Errorf("missing session")
	}
	raw, err := open(s.SyncSecret, c.Value)
	if err != nil {
		return nil, err
	}
	var sess setupSession
	if err := json.Unmarshal(raw, &sess); err != nil {
		return nil, err
	}
	if sess.Exp > 0 && time.Now().Unix() > sess.Exp {
		return nil, fmt.Errorf("session expired")
	}
	if strings.TrimSpace(sess.TokenJSON) == "" {
		return nil, fmt.Errorf("empty session")
	}
	return &sess, nil
}

func randomState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func seal(secret string, plaintext []byte) (string, error) {
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.RawURLEncoding.EncodeToString(out), nil
}

func open(secret, encoded string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
