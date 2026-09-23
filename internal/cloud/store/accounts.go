package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/saugatadhikari/jobSync/internal/cloud/secret"
	"github.com/saugatadhikari/jobSync/internal/config"
	"github.com/saugatadhikari/jobSync/internal/domain"
)

// UpsertAccountFromLocal saves local CLI config + OAuth token to Neon.
// Gemini API keys and OAuth tokens are encrypted at rest.
func (s *Store) UpsertAccountFromLocal(ctx context.Context, cfg *config.Config, tokenJSON []byte) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	now := time.Now().UTC()
	model := cfg.GeminiModel
	if model == "" {
		model = config.DefaultGeminiModel
	}
	sheetName := cfg.SheetName
	if sheetName == "" {
		sheetName = config.DefaultSheetName
	}

	encKey := secret.KeyFromEnv()
	geminiEnc, err := secret.Encrypt(encKey, cfg.GeminiAPIKey)
	if err != nil {
		return fmt.Errorf("encrypt gemini key: %w", err)
	}
	tokenEnc, err := secret.Encrypt(encKey, string(tokenJSON))
	if err != nil {
		return fmt.Errorf("encrypt oauth token: %w", err)
	}

	_, err = s.SQL.ExecContext(ctx, `
		INSERT INTO accounts (
			id, spreadsheet_id, sheet_name, gemini_api_key, gemini_model,
			oauth_token_json, auth_scopes_version,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$8)
		ON CONFLICT (id) DO UPDATE SET
			spreadsheet_id = EXCLUDED.spreadsheet_id,
			sheet_name = EXCLUDED.sheet_name,
			gemini_api_key = EXCLUDED.gemini_api_key,
			gemini_model = EXCLUDED.gemini_model,
			oauth_token_json = EXCLUDED.oauth_token_json,
			auth_scopes_version = EXCLUDED.auth_scopes_version,
			updated_at = EXCLUDED.updated_at`,
		s.AccountID,
		cfg.SpreadsheetID,
		sheetName,
		geminiEnc,
		model,
		tokenEnc,
		cfg.AuthScopesVersion,
		now,
	)
	if err != nil {
		return fmt.Errorf("upsert account: %w", err)
	}
	return nil
}

// GetAccount loads cloud account settings.
func (s *Store) GetAccount(ctx context.Context) (*domain.Account, error) {
	row := s.SQL.QueryRowContext(ctx, `
		SELECT id, spreadsheet_id, sheet_name, gemini_api_key, gemini_model,
			oauth_token_json, auth_scopes_version,
			created_at, updated_at
		FROM accounts WHERE id = $1`, s.AccountID)
	return scanAccount(row)
}

// SaveOAuthToken updates the stored Google OAuth token (after refresh).
func (s *Store) SaveOAuthToken(ctx context.Context, tokenJSON []byte) error {
	tokenEnc, err := secret.Encrypt(secret.KeyFromEnv(), string(tokenJSON))
	if err != nil {
		return fmt.Errorf("encrypt oauth token: %w", err)
	}
	res, err := s.SQL.ExecContext(ctx, `
		UPDATE accounts SET oauth_token_json = $1, updated_at = $2 WHERE id = $3`,
		tokenEnc, time.Now().UTC(), s.AccountID,
	)
	if err != nil {
		return fmt.Errorf("save oauth token: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("account %q not found — run jobsync cloud push", s.AccountID)
	}
	return nil
}

func scanAccount(row *sql.Row) (*domain.Account, error) {
	var acc domain.Account
	err := row.Scan(
		&acc.ID, &acc.SpreadsheetID, &acc.SheetName, &acc.GeminiAPIKey, &acc.GeminiModel,
		&acc.OAuthTokenJSON, &acc.AuthScopesVersion,
		&acc.CreatedAt, &acc.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan account: %w", err)
	}
	if err := decryptAccountSecrets(&acc); err != nil {
		return nil, err
	}
	if acc.GeminiModel == "" {
		acc.GeminiModel = config.DefaultGeminiModel
	}
	return &acc, nil
}

func decryptAccountSecrets(acc *domain.Account) error {
	if acc == nil {
		return nil
	}
	key := secret.KeyFromEnv()
	gemini, err := secret.Decrypt(key, acc.GeminiAPIKey)
	if err != nil {
		return fmt.Errorf("decrypt gemini key: %w", err)
	}
	token, err := secret.Decrypt(key, acc.OAuthTokenJSON)
	if err != nil {
		return fmt.Errorf("decrypt oauth token: %w", err)
	}
	acc.GeminiAPIKey = gemini
	acc.OAuthTokenJSON = token
	return nil
}

func (s *Store) MarkEmailProcessed(ctx context.Context, rec domain.EmailProcessed) error {
	if rec.ProcessedAt.IsZero() {
		rec.ProcessedAt = time.Now().UTC()
	}
	_, err := s.SQL.ExecContext(ctx, `
		INSERT INTO email_processed (account_id, gmail_message_id, processed_at, classification)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (account_id, gmail_message_id) DO UPDATE SET
			processed_at = EXCLUDED.processed_at,
			classification = EXCLUDED.classification`,
		s.AccountID, rec.GmailMessageID,
		rec.ProcessedAt.UTC(), rec.Classification,
	)
	if err != nil {
		return fmt.Errorf("mark email processed: %w", err)
	}
	return nil
}

func (s *Store) IsEmailProcessed(ctx context.Context, gmailMessageID string) (bool, error) {
	var one int
	err := s.SQL.QueryRowContext(ctx,
		`SELECT 1 FROM email_processed WHERE account_id = $1 AND gmail_message_id = $2`,
		s.AccountID, gmailMessageID,
	).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) CreateSyncRun(ctx context.Context, run *domain.SyncRun) error {
	if run.ID == "" {
		run.ID = uuid.NewString()
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = time.Now().UTC()
	}
	_, err := s.SQL.ExecContext(ctx, `
		INSERT INTO sync_runs (
			id, account_id, started_at, finished_at, status,
			emails_seen, emails_updated, errors,
			gemini_calls, gemini_skipped_prefilter,
			watermark, error_summary
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		run.ID, s.AccountID,
		run.StartedAt.UTC(), formatTime(run.FinishedAt), run.Status,
		run.EmailsSeen, run.EmailsUpdated, run.Errors,
		run.GeminiCalls, run.GeminiSkippedPrefilter,
		run.Watermark, run.ErrorSummary,
	)
	if err != nil {
		return fmt.Errorf("create sync run: %w", err)
	}
	return nil
}

func (s *Store) FinishSyncRun(ctx context.Context, run *domain.SyncRun) error {
	now := time.Now().UTC()
	run.FinishedAt = &now
	_, err := s.SQL.ExecContext(ctx, `
		UPDATE sync_runs SET
			finished_at = $1, status = $2,
			emails_seen = $3, emails_updated = $4, errors = $5,
			gemini_calls = $6, gemini_skipped_prefilter = $7,
			watermark = $8, error_summary = $9
		WHERE id = $10 AND account_id = $11`,
		run.FinishedAt.UTC(), run.Status,
		run.EmailsSeen, run.EmailsUpdated, run.Errors,
		run.GeminiCalls, run.GeminiSkippedPrefilter,
		run.Watermark, run.ErrorSummary,
		run.ID, s.AccountID,
	)
	if err != nil {
		return fmt.Errorf("finish sync run: %w", err)
	}
	return nil
}

func (s *Store) GetLastSuccessfulWatermark(ctx context.Context) (string, error) {
	var watermark string
	err := s.SQL.QueryRowContext(ctx, `
		SELECT watermark FROM sync_runs
		WHERE account_id = $1 AND status IN ($2, $3) AND watermark != ''
		ORDER BY started_at DESC
		LIMIT 1`,
		s.AccountID, domain.SyncStatusSuccess, domain.SyncStatusPartial,
	).Scan(&watermark)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return watermark, nil
}

func formatTime(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.UTC()
}
