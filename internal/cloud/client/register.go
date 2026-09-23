package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type RegisterRequest struct {
	SpreadsheetID     string `json:"spreadsheet_id"`
	SheetName         string `json:"sheet_name"`
	GeminiAPIKey      string `json:"gemini_api_key"`
	GeminiModel       string `json:"gemini_model"`
	OAuthTokenJSON    string `json:"oauth_token_json"`
	AuthScopesVersion int    `json:"auth_scopes_version"`
}

type RegisterResponse struct {
	AccountID string `json:"account_id"`
	Status    string `json:"status"`
}

type SetupCompleteResponse struct {
	AccountID      string `json:"account_id"`
	SpreadsheetID  string `json:"spreadsheet_id"`
	SpreadsheetURL string `json:"spreadsheet_url"`
	Status         string `json:"status"`
	ReusedSheet    bool   `json:"reused_sheet"`
	SyncStatus     string `json:"sync_status,omitempty"`
	EmailsCreated  int    `json:"emails_created,omitempty"`
	EmailsUpdated  int    `json:"emails_updated,omitempty"`
	QuotaExhausted bool   `json:"quota_exhausted,omitempty"`
	SyncError      string `json:"sync_error,omitempty"`
}

type LookupRequest struct {
	OAuthTokenJSON string `json:"oauth_token_json"`
}

type LookupResponse struct {
	Found         bool   `json:"found"`
	AccountID     string `json:"account_id,omitempty"`
	SpreadsheetID string `json:"spreadsheet_id,omitempty"`
	SheetName     string `json:"sheet_name,omitempty"`
	HasGeminiKey  bool   `json:"has_gemini_key,omitempty"`
}

func Register(ctx context.Context, serverURL string, req RegisterRequest) (*RegisterResponse, error) {
	var out RegisterResponse
	if err := postJSON(ctx, serverURL, "/register", 2*time.Minute, req, &out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.AccountID) == "" {
		return nil, fmt.Errorf("register response missing account_id")
	}
	return &out, nil
}

func Lookup(ctx context.Context, serverURL string, oauthTokenJSON string) (*LookupResponse, error) {
	var out LookupResponse
	if err := postJSON(ctx, serverURL, "/lookup", 30*time.Second, LookupRequest{OAuthTokenJSON: oauthTokenJSON}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func postJSON(ctx context.Context, serverURL, path string, timeout time.Duration, payload, out any) error {
	serverURL = strings.TrimRight(strings.TrimSpace(serverURL), "/")
	if serverURL == "" {
		return fmt.Errorf("cloud server URL is empty")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serverURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimPrefix(path, "/"), err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("%s failed (%d): %s", strings.TrimPrefix(path, "/"), resp.StatusCode, msg)
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode %s response: %w", strings.TrimPrefix(path, "/"), err)
	}
	return nil
}
