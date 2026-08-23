package service

import (
	"context"
	"fmt"

	"github.com/saugatadhikari/jobSync/internal/cloud/store"
	"github.com/saugatadhikari/jobSync/internal/domain"
	"github.com/saugatadhikari/jobSync/internal/gemini"
	"github.com/saugatadhikari/jobSync/internal/google/auth"
	"github.com/saugatadhikari/jobSync/internal/google/gmail"
	"github.com/saugatadhikari/jobSync/internal/google/sheets"
	"github.com/saugatadhikari/jobSync/internal/syncer"
)

const DefaultSyncLimit = 15

// RunCloudSync executes one sync for a Neon-backed account (Cloud Run).
func RunCloudSync(ctx context.Context, st *store.Store, acc *domain.Account, limit int64, dryRun bool, logf func(string, ...any)) (*syncer.Result, error) {
	if st == nil || acc == nil {
		return nil, fmt.Errorf("store and account are required")
	}
	if !acc.HasGeminiKey() {
		return nil, fmt.Errorf("account missing gemini_api_key — run jobsync cloud push")
	}
	if !acc.HasSpreadsheet() {
		return nil, fmt.Errorf("account missing spreadsheet_id — run jobsync cloud push")
	}
	if !acc.HasOAuthToken() {
		return nil, fmt.Errorf("account missing oauth token — run jobsync cloud push")
	}
	if acc.AuthScopesVersion < auth.CurrentScopesVersion {
		return nil, fmt.Errorf("google oauth scopes outdated — run jobsync init then jobsync cloud push")
	}
	if limit <= 0 {
		limit = DefaultSyncLimit
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}

	httpClient, err := auth.HTTPClientFromStoredToken(ctx, acc.OAuthTokenJSON, func(data []byte) error {
		return st.SaveOAuthToken(ctx, data)
	})
	if err != nil {
		return nil, fmt.Errorf("google auth: %w", err)
	}

	gclient, err := gmail.NewClient(ctx, httpClient)
	if err != nil {
		return nil, err
	}
	geminiClient, err := gemini.NewClient(gemini.Options{
		APIKey: acc.GeminiAPIKey,
		Model:  acc.GeminiModel,
	})
	if err != nil {
		return nil, err
	}
	sheetsClient, err := sheets.NewClient(ctx, httpClient, acc.SpreadsheetID, acc.SheetName)
	if err != nil {
		return nil, err
	}

	runner := &syncer.Runner{
		Gmail:  gclient,
		Gemini: geminiClient,
		Sheets: sheetsClient,
		DB:     st,
		Log:    logf,
	}
	return runner.Run(ctx, syncer.Options{
		Limit:  limit,
		DryRun: dryRun,
	})
}
