package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/saugatadhikari/jobSync/internal/cloud/client"
	"github.com/saugatadhikari/jobSync/internal/cloud/store"
	"github.com/saugatadhikari/jobSync/internal/config"
	"github.com/saugatadhikari/jobSync/internal/google/auth"
)

// RegisterAccount saves or updates a user's cloud sync credentials in Neon.
// Account id is always derived from the verified Google OAuth token.
// If this Gmail already has a spreadsheet_id, that sheet is kept (not replaced).
func RegisterAccount(ctx context.Context, db *store.DB, req client.RegisterRequest) (string, error) {
	if db == nil {
		return "", fmt.Errorf("database is nil")
	}

	spreadsheetID := strings.TrimSpace(req.SpreadsheetID)
	geminiKey := strings.TrimSpace(req.GeminiAPIKey)
	if geminiKey == "" {
		return "", fmt.Errorf("gemini_api_key is required")
	}
	tokenJSON := strings.TrimSpace(req.OAuthTokenJSON)
	if tokenJSON == "" {
		return "", fmt.Errorf("oauth_token_json is required")
	}
	if _, err := auth.TokenFromJSON([]byte(tokenJSON)); err != nil {
		return "", fmt.Errorf("invalid oauth token: %w", err)
	}

	email, err := auth.GoogleEmailFromTokenJSON(ctx, []byte(tokenJSON))
	if err != nil {
		return "", fmt.Errorf("verify google account: %w", err)
	}
	accountID := auth.AccountIDFromEmail(email)

	sheetName := strings.TrimSpace(req.SheetName)
	if sheetName == "" {
		sheetName = config.DefaultSheetName
	}
	model := strings.TrimSpace(req.GeminiModel)
	if model == "" {
		model = config.DefaultGeminiModel
	}

	st := db.Store(accountID)
	existing, err := st.GetAccount(ctx)
	if err != nil {
		return "", err
	}
	if existing != nil && strings.TrimSpace(existing.SpreadsheetID) != "" {
		// Same Gmail → keep the tracker sheet already linked in the cloud.
		spreadsheetID = strings.TrimSpace(existing.SpreadsheetID)
		if strings.TrimSpace(existing.SheetName) != "" {
			sheetName = strings.TrimSpace(existing.SheetName)
		}
	}
	if spreadsheetID == "" {
		return "", fmt.Errorf("spreadsheet_id is required")
	}

	cfg := &config.Config{
		SpreadsheetID:     spreadsheetID,
		SheetName:         sheetName,
		GeminiAPIKey:      geminiKey,
		GeminiModel:       model,
		AuthScopesVersion: req.AuthScopesVersion,
	}

	if err := st.UpsertAccountFromLocal(ctx, cfg, []byte(tokenJSON)); err != nil {
		return "", err
	}
	return accountID, nil
}

// LookupAccount returns cloud registration for the Google account in tokenJSON.
// Found is false when this Gmail has never registered (web or cloud push).
func LookupAccount(ctx context.Context, db *store.DB, tokenJSON string) (*client.LookupResponse, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	tokenJSON = strings.TrimSpace(tokenJSON)
	if tokenJSON == "" {
		return nil, fmt.Errorf("oauth_token_json is required")
	}
	if _, err := auth.TokenFromJSON([]byte(tokenJSON)); err != nil {
		return nil, fmt.Errorf("invalid oauth token: %w", err)
	}
	email, err := auth.GoogleEmailFromTokenJSON(ctx, []byte(tokenJSON))
	if err != nil {
		return nil, fmt.Errorf("verify google account: %w", err)
	}
	accountID := auth.AccountIDFromEmail(email)
	acc, err := db.Store(accountID).GetAccount(ctx)
	if err != nil {
		return nil, err
	}
	if acc == nil || strings.TrimSpace(acc.SpreadsheetID) == "" {
		return &client.LookupResponse{Found: false, AccountID: accountID}, nil
	}
	sheetName := strings.TrimSpace(acc.SheetName)
	if sheetName == "" {
		sheetName = config.DefaultSheetName
	}
	return &client.LookupResponse{
		Found:         true,
		AccountID:     accountID,
		SpreadsheetID: strings.TrimSpace(acc.SpreadsheetID),
		SheetName:     sheetName,
		HasGeminiKey:  strings.TrimSpace(acc.GeminiAPIKey) != "",
	}, nil
}
