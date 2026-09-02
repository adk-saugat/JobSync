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

// RegisterRequest is sent by the CLI to POST /register.
type RegisterRequest struct {
	SpreadsheetID     string `json:"spreadsheet_id"`
	SheetName         string `json:"sheet_name"`
	GeminiAPIKey      string `json:"gemini_api_key"`
	GeminiModel       string `json:"gemini_model"`
	OAuthTokenJSON    string `json:"oauth_token_json"`
	AuthScopesVersion int    `json:"auth_scopes_version"`
}

// RegisterResponse is returned after a successful registration.
type RegisterResponse struct {
	AccountID string `json:"account_id"`
	Status    string `json:"status"`
}

// SetupCompleteResponse is returned by the web setup wizard.
type SetupCompleteResponse struct {
	AccountID      string `json:"account_id"`
	SpreadsheetID  string `json:"spreadsheet_id"`
	SpreadsheetURL string `json:"spreadsheet_url"`
	Status         string `json:"status"`
	ReusedSheet    bool   `json:"reused_sheet"`
}

// Register posts account credentials to the hosted JobSync server.
// The Google OAuth token from init proves identity — no shared secret required.
func Register(ctx context.Context, serverURL string, req RegisterRequest) (*RegisterResponse, error) {
	serverURL = strings.TrimRight(strings.TrimSpace(serverURL), "/")
	if serverURL == "" {
		return nil, fmt.Errorf("cloud server URL is empty")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, serverURL+"/register", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("register request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("register failed (%d): %s", resp.StatusCode, msg)
	}

	var out RegisterResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("decode register response: %w", err)
	}
	if strings.TrimSpace(out.AccountID) == "" {
		return nil, fmt.Errorf("register response missing account_id")
	}
	return &out, nil
}
