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
func RegisterAccount(ctx context.Context, db *store.DB, req client.RegisterRequest) (string, error) {
	if db == nil {
		return "", fmt.Errorf("database is nil")
	}

	spreadsheetID := strings.TrimSpace(req.SpreadsheetID)
	if spreadsheetID == "" {
		return "", fmt.Errorf("spreadsheet_id is required")
	}
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

	cfg := &config.Config{
		SpreadsheetID:     spreadsheetID,
		SheetName:         sheetName,
		GeminiAPIKey:      geminiKey,
		GeminiModel:       model,
		AuthScopesVersion: req.AuthScopesVersion,
	}

	st := db.Store(accountID)
	if err := st.UpsertAccountFromLocal(ctx, cfg, []byte(tokenJSON)); err != nil {
		return "", err
	}
	return accountID, nil
}
