package service

import (
	"context"
	"fmt"
	"log"
	"strings"

	"golang.org/x/oauth2"

	"github.com/saugatadhikari/jobSync/internal/cloud/client"
	"github.com/saugatadhikari/jobSync/internal/cloud/store"
	"github.com/saugatadhikari/jobSync/internal/config"
	"github.com/saugatadhikari/jobSync/internal/google/auth"
	"github.com/saugatadhikari/jobSync/internal/google/sheets"
	"github.com/saugatadhikari/jobSync/internal/syncer"
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
	out := &client.SetupCompleteResponse{
		AccountID:      registeredID,
		SpreadsheetID:  spreadsheetID,
		SpreadsheetURL: sheets.SpreadsheetURL(spreadsheetID),
		Status:         status,
		ReusedSheet:    reused,
	}

	syncRes, syncErr := RunCloudSyncForAccount(ctx, db, registeredID, DefaultSyncLimit, false, func(format string, args ...any) {
		log.Printf("setup sync: "+format, args...)
	})
	attachSetupSync(out, syncRes, syncErr)
	return out, nil
}

func attachSetupSync(out *client.SetupCompleteResponse, res *syncer.Result, err error) {
	if out == nil {
		return
	}
	if err != nil {
		out.SyncStatus = "failed"
		out.SyncError = err.Error()
		return
	}
	if res == nil {
		return
	}
	out.SyncStatus = res.Status
	out.EmailsCreated = res.EmailsCreated
	out.EmailsUpdated = res.EmailsUpdated
	out.QuotaExhausted = res.QuotaExhausted
}

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
