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

// CompleteWebSetup registers the account for cloud sync.
// If this Gmail already has a tracker sheet in Neon, that sheet is reused;
// otherwise a new JobSync Tracker spreadsheet is created.
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

	email, err := auth.GoogleEmailFromTokenJSON(ctx, tokenJSON)
	if err != nil {
		return nil, fmt.Errorf("verify google account: %w", err)
	}
	accountID := auth.AccountIDFromEmail(email)

	oauthCfg, err := auth.LoadOAuthConfig(auth.RequiredScopes)
	if err != nil {
		return nil, err
	}
	src := oauthCfg.TokenSource(ctx, tok)
	httpClient := oauth2.NewClient(ctx, src)

	sheetName := config.DefaultSheetName
	spreadsheetID := ""
	reused := false

	if existing, err := db.Store(accountID).GetAccount(ctx); err != nil {
		return nil, err
	} else if existing != nil && strings.TrimSpace(existing.SpreadsheetID) != "" {
		spreadsheetID = strings.TrimSpace(existing.SpreadsheetID)
		if strings.TrimSpace(existing.SheetName) != "" {
			sheetName = strings.TrimSpace(existing.SheetName)
		}
		reused = true
		client, err := sheets.NewClient(ctx, httpClient, spreadsheetID, sheetName)
		if err != nil {
			return nil, fmt.Errorf("open existing spreadsheet: %w", err)
		}
		if err := client.SetupSheet(ctx); err != nil {
			return nil, fmt.Errorf("refresh existing spreadsheet: %w", err)
		}
	} else {
		spreadsheetID, err = sheets.CreateTrackerSpreadsheet(ctx, httpClient, "JobSync Tracker", sheetName)
		if err != nil {
			return nil, fmt.Errorf("create spreadsheet: %w", err)
		}
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

	registeredID, err := RegisterAccount(ctx, db, client.RegisterRequest{
		SpreadsheetID:     spreadsheetID,
		SheetName:         sheetName,
		GeminiAPIKey:      geminiAPIKey,
		GeminiModel:       config.DefaultGeminiModel,
		OAuthTokenJSON:    string(persisted),
		AuthScopesVersion: auth.CurrentScopesVersion,
	})
	if err != nil {
		return nil, err
	}

	status := "registered"
	if reused {
		status = "updated"
	}
	return &client.SetupCompleteResponse{
		AccountID:      registeredID,
		SpreadsheetID:  spreadsheetID,
		SpreadsheetURL: sheets.SpreadsheetURL(spreadsheetID),
		Status:         status,
		ReusedSheet:    reused,
	}, nil
}
