package service

import (
	"context"
	"fmt"

	"github.com/saugatadhikari/jobSync/internal/cloud/store"
	"github.com/saugatadhikari/jobSync/internal/domain"
	"github.com/saugatadhikari/jobSync/internal/syncer"
)

// AccountSyncResult is one account's sync outcome.
type AccountSyncResult struct {
	AccountID string         `json:"account_id"`
	Result    *syncer.Result `json:"result,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// SyncAllResult summarizes a multi-tenant cloud run.
type SyncAllResult struct {
	Accounts int                 `json:"accounts"`
	Results  []AccountSyncResult `json:"results"`
}

// RunCloudSyncAll syncs every account registered in Neon.
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

// RunCloudSyncForAccount loads one account by id and syncs it.
func RunCloudSyncForAccount(ctx context.Context, db *store.DB, accountID string, limit int64, dryRun bool, logf func(string, ...any)) (*syncer.Result, error) {
	st := db.Store(accountID)
	acc, err := st.GetAccount(ctx)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, fmt.Errorf("account %q not found — run jobsync cloud push", accountID)
	}
	return RunCloudSync(ctx, st, acc, limit, dryRun, logf)
}

// FirstAccountError returns the first sync error from a multi-run, if any.
func FirstAccountError(summary *SyncAllResult) error {
	if summary == nil {
		return nil
	}
	for _, r := range summary.Results {
		if r.Error != "" {
			return fmt.Errorf("%s: %s", r.AccountID, r.Error)
		}
		if r.Result != nil && r.Result.Status == domain.SyncStatusFailed {
			return fmt.Errorf("%s: sync failed", r.AccountID)
		}
	}
	return nil
}
