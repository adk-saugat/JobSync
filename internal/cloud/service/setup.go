package service

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/oauth2"

	"github.com/saugatadhikari/jobSync/internal/cloud/client"
	"github.com/saugatadhikari/jobSync/internal/cloud/store"
	"github.com/saugatadhikari/jobSync/internal/config"
	"github.com/saugatadhikari/jobSync/internal/google/auth"
	"github.com/saugatadhikari/jobSync/internal/google/sheets"
)

// CompleteWebSetup creates a tracker sheet and registers the account for cloud sync.
func CompleteWebSetup(ctx context.Context, db *store.DB, tokenJSON []byte, geminiAPIKey string) (*client.SetupCompleteResponse, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	geminiAPIKey = strings.TrimSpace(geminiAPIKey)
	if geminiAPIKey == "" {
		return nil, fmt.Errorf("gemini_api_key is required")
	}
	tok, err := auth.TokenFromJSON(tokenJSON)
	if err != nil {
		return nil, fmt.Errorf("invalid oauth token: %w", err)
	}
	if strings.TrimSpace(tok.RefreshToken) == "" {
		return nil, fmt.Errorf("missing refresh token — sign in again and grant offline access")
	}

	oauthCfg, err := auth.LoadOAuthConfig(auth.RequiredScopes)
	if err != nil {
		return nil, err
	}
	src := oauthCfg.TokenSource(ctx, tok)
	httpClient := oauth2.NewClient(ctx, src)

	spreadsheetID, err := sheets.CreateTrackerSpreadsheet(ctx, httpClient, "JobSync Tracker", config.DefaultSheetName)
	if err != nil {
		return nil, fmt.Errorf("create spreadsheet: %w", err)
	}

	latest := tok
	if refreshed, err := src.Token(); err == nil && refreshed != nil {
		latest = refreshed
		if latest.RefreshToken == "" {
			latest.RefreshToken = tok.RefreshToken
		}
	}
	persisted, err := auth.MarshalTokenJSON(latest)
	if err != nil {
		return nil, err
	}

	accountID, err := RegisterAccount(ctx, db, client.RegisterRequest{
		SpreadsheetID:     spreadsheetID,
		SheetName:         config.DefaultSheetName,
		GeminiAPIKey:      geminiAPIKey,
		GeminiModel:       config.DefaultGeminiModel,
		OAuthTokenJSON:    string(persisted),
		AuthScopesVersion: auth.CurrentScopesVersion,
	})
	if err != nil {
		return nil, err
	}

	return &client.SetupCompleteResponse{
		AccountID:      accountID,
		SpreadsheetID:  spreadsheetID,
		SpreadsheetURL: sheets.SpreadsheetURL(spreadsheetID),
		Status:         "registered",
	}, nil
}
