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
		return nil, fmt.Errorf("account missing gemini_api_key")
	}
	if !acc.HasSpreadsheet() {
		return nil, fmt.Errorf("account missing spreadsheet_id")
	}
	if !acc.HasOAuthToken() {
		return nil, fmt.Errorf("account missing oauth token")
	}
	if acc.AuthScopesVersion < auth.CurrentScopesVersion {
		return nil, fmt.Errorf("google oauth scopes outdated — sign in with Google again")
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

type AccountSyncResult struct {
	AccountID string         `json:"account_id"`
	Result    *syncer.Result `json:"result,omitempty"`
	Error     string         `json:"error,omitempty"`
}

type SyncAllResult struct {
	Accounts int                 `json:"accounts"`
	Results  []AccountSyncResult `json:"results"`
}

func RunCloudSyncAll(ctx context.Context, db *store.DB, limit int64, dryRun bool, logf func(string, ...any)) (*SyncAllResult, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required")
	}
	ids, err := db.ListAccountIDs(ctx)
	if err != nil {
		return nil, err
	}
	out := &SyncAllResult{Accounts: len(ids)}
	for _, id := range ids {
		st := db.Store(id)
		acc, err := st.GetAccount(ctx)
		if err != nil {
			out.Results = append(out.Results, AccountSyncResult{AccountID: id, Error: err.Error()})
			continue
		}
		if acc == nil {
			out.Results = append(out.Results, AccountSyncResult{AccountID: id, Error: "account not found"})
			continue
		}
		res, err := RunCloudSync(ctx, st, acc, limit, dryRun, func(format string, args ...any) {
			if logf != nil {
				logf("[%s] "+format, append([]any{id}, args...)...)
			}
		})
		item := AccountSyncResult{AccountID: id, Result: res}
		if err != nil {
			item.Error = err.Error()
		}
		out.Results = append(out.Results, item)
	}
	return out, nil
}

func RunCloudSyncForAccount(ctx context.Context, db *store.DB, accountID string, limit int64, dryRun bool, logf func(string, ...any)) (*syncer.Result, error) {
	st := db.Store(accountID)
	acc, err := st.GetAccount(ctx)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, fmt.Errorf("account %q not found", accountID)
	}
	return RunCloudSync(ctx, st, acc, limit, dryRun, logf)
}
