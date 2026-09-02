package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/saugatadhikari/jobSync/internal/cloud/client"
	"github.com/saugatadhikari/jobSync/internal/cloud/service"
	"github.com/saugatadhikari/jobSync/internal/cloud/store"
)

// Server is the Cloud Run HTTP API.
type Server struct {
	SyncSecret string
	DB         *store.DB
}

// NewFromEnv builds a server from environment variables.
func NewFromEnv(ctx context.Context) (*Server, error) {
	secret := os.Getenv("SYNC_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("SYNC_SECRET is required")
	}
	db, err := store.OpenFromEnv(ctx)
	if err != nil {
		return nil, err
	}
	return &Server{
		SyncSecret: secret,
		DB:         db,
	}, nil
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /register", s.handleRegister)
	mux.HandleFunc("POST /lookup", s.handleLookup)
	mux.HandleFunc("POST /sync", s.handleSync)
	mux.HandleFunc("POST /sync/all", s.handleSyncAll)

	mux.HandleFunc("GET /setup/oauth/start", s.handleSetupOAuthStart)
	mux.HandleFunc("GET /setup/oauth/callback", s.handleSetupOAuthCallback)
	mux.HandleFunc("GET /setup/session", s.handleSetupSession)
	mux.HandleFunc("POST /setup/complete", s.handleSetupComplete)

	spa := s.spaHandler()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		spa.ServeHTTP(w, r)
	})
	return mux
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req client.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	accountID, err := service.RegisterAccount(ctx, s.DB, req)
	if err != nil {
		log.Printf("register error: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, client.RegisterResponse{
		AccountID: accountID,
		Status:    "registered",
	})
	log.Printf("register ok account=%s", accountID)
}

func (s *Server) handleLookup(w http.ResponseWriter, r *http.Request) {
	var req client.LookupRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	out, err := service.LookupAccount(ctx, s.DB, req.OAuthTokenJSON)
	if err != nil {
		log.Printf("lookup error: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	if !s.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	limit, dryRun, err := parseSyncQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	accountID := strings.TrimSpace(r.URL.Query().Get("account_id"))
	if accountID == "" {
		accountID = strings.TrimSpace(os.Getenv("ACCOUNT_ID"))
	}
	if accountID == "" {
		http.Error(w, "account_id query param required (or set ACCOUNT_ID for single-tenant)", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 9*time.Minute)
	defer cancel()

	log.Printf("sync start account=%s limit=%d dry_run=%v", accountID, limit, dryRun)
	res, err := service.RunCloudSyncForAccount(ctx, s.DB, accountID, limit, dryRun, func(format string, args ...any) {
		log.Printf("sync: "+format, args...)
	})
	if err != nil {
		log.Printf("sync error account=%s: %v", accountID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	status := http.StatusOK
	if res != nil && res.QuotaExhausted {
		status = http.StatusAccepted
	}
	writeJSON(w, status, res)
	log.Printf("sync done account=%s status=%v gemini_calls=%d updated=%d", accountID, res.Status, res.GeminiCalls, res.EmailsUpdated)
}

func (s *Server) handleSyncAll(w http.ResponseWriter, r *http.Request) {
	if !s.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	limit, dryRun, err := parseSyncQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 9*time.Minute)
	defer cancel()

	log.Printf("sync/all start limit=%d dry_run=%v", limit, dryRun)
	summary, err := service.RunCloudSyncAll(ctx, s.DB, limit, dryRun, func(format string, args ...any) {
		log.Printf("sync: "+format, args...)
	})
	if err != nil {
		log.Printf("sync/all error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	status := http.StatusOK
	for _, item := range summary.Results {
		if item.Result != nil && item.Result.QuotaExhausted {
			status = http.StatusAccepted
			break
		}
	}
	writeJSON(w, status, summary)
	log.Printf("sync/all done accounts=%d", summary.Accounts)
}

func parseSyncQuery(r *http.Request) (limit int64, dryRun bool, err error) {
	limit = int64(service.DefaultSyncLimit)
	if v := r.URL.Query().Get("limit"); v != "" {
		n, parseErr := strconv.ParseInt(v, 10, 64)
		if parseErr != nil || n < 1 {
			return 0, false, fmt.Errorf("invalid limit")
		}
		limit = n
	}
	dryRun = strings.EqualFold(r.URL.Query().Get("dry_run"), "true")
	return limit, dryRun, nil
}

func (s *Server) authorize(r *http.Request) bool {
	if got := strings.TrimSpace(r.Header.Get("X-Sync-Secret")); got != "" {
		return got == s.SyncSecret
	}
	got := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(got), "bearer ") {
		got = strings.TrimSpace(got[7:])
	}
	return got != "" && got == s.SyncSecret
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
