package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/register" {
			http.NotFound(w, r)
			return
		}
		var req RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, RegisterResponse{AccountID: "jane_at_gmail.com", Status: "registered"})
	}))
	defer srv.Close()

	resp, err := Register(context.Background(), srv.URL, RegisterRequest{
		SpreadsheetID:  "sheet",
		GeminiAPIKey:   "key",
		OAuthTokenJSON: `{"access_token":"x","refresh_token":"y","token_type":"Bearer"}`,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if resp.AccountID != "jane_at_gmail.com" {
		t.Fatalf("AccountID = %q", resp.AccountID)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
