package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/saugatadhikari/jobSync/internal/domain"
)

const timeLayout = time.RFC3339

// CreateApplication inserts a new application row.
func (db *DB) CreateApplication(ctx context.Context, app *domain.Application) error {
	now := time.Now().UTC()
	if app.ID == "" {
		app.ID = uuid.NewString()
	}
	if app.CreatedAt.IsZero() {
		app.CreatedAt = now
	}
	if app.UpdatedAt.IsZero() {
		app.UpdatedAt = now
	}

	_, err := db.SQL.ExecContext(ctx, `
		INSERT INTO applications (
			id, company, position, status,
			applied_at, interview_at, oa_at,
			source_email_id, sheet_row_id, raw_excerpt,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		app.ID,
		app.Company,
		app.Position,
		app.Status,
		formatTime(app.AppliedAt),
		formatTime(app.InterviewAt),
		formatTime(app.OAAt),
		nullIfEmpty(app.SourceEmailID),
		nullIfEmpty(app.SheetRowID),
		app.RawExcerpt,
		app.CreatedAt.UTC().Format(timeLayout),
		app.UpdatedAt.UTC().Format(timeLayout),
	)
	if err != nil {
		return fmt.Errorf("create application: %w", err)
	}
	return nil
}

// GetApplicationByID loads an application by id.
func (db *DB) GetApplicationByID(ctx context.Context, id string) (*domain.Application, error) {
	row := db.SQL.QueryRowContext(ctx, `
		SELECT id, company, position, status,
			applied_at, interview_at, oa_at,
			source_email_id, sheet_row_id, raw_excerpt,
			created_at, updated_at
		FROM applications WHERE id = ?`, id)
	return scanApplication(row)
}

// FindBySourceEmailID finds an application by Gmail message id.
func (db *DB) FindBySourceEmailID(ctx context.Context, emailID string) (*domain.Application, error) {
	row := db.SQL.QueryRowContext(ctx, `
		SELECT id, company, position, status,
			applied_at, interview_at, oa_at,
			source_email_id, sheet_row_id, raw_excerpt,
			created_at, updated_at
		FROM applications WHERE source_email_id = ?`, emailID)
	return scanApplication(row)
}

// FindByCompanyAndPosition finds by case-insensitive company + position.
func (db *DB) FindByCompanyAndPosition(ctx context.Context, company, position string) (*domain.Application, error) {
	row := db.SQL.QueryRowContext(ctx, `
		SELECT id, company, position, status,
			applied_at, interview_at, oa_at,
			source_email_id, sheet_row_id, raw_excerpt,
			created_at, updated_at
		FROM applications
		WHERE company = ? COLLATE NOCASE AND position = ? COLLATE NOCASE
		LIMIT 1`, company, position)
	return scanApplication(row)
}

// UpdateApplication updates mutable application fields.
func (db *DB) UpdateApplication(ctx context.Context, app *domain.Application) error {
	app.UpdatedAt = time.Now().UTC()
	res, err := db.SQL.ExecContext(ctx, `
		UPDATE applications SET
			company = ?, position = ?, status = ?,
			applied_at = ?, interview_at = ?, oa_at = ?,
			source_email_id = ?, sheet_row_id = ?, raw_excerpt = ?,
			updated_at = ?
		WHERE id = ?`,
		app.Company,
		app.Position,
		app.Status,
		formatTime(app.AppliedAt),
		formatTime(app.InterviewAt),
		formatTime(app.OAAt),
		nullIfEmpty(app.SourceEmailID),
		nullIfEmpty(app.SheetRowID),
		app.RawExcerpt,
		app.UpdatedAt.Format(timeLayout),
		app.ID,
	)
	if err != nil {
		return fmt.Errorf("update application: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("update application: not found")
	}
	return nil
}

// MarkEmailProcessed records that a Gmail message was handled.
func (db *DB) MarkEmailProcessed(ctx context.Context, rec domain.EmailProcessed) error {
	if rec.ProcessedAt.IsZero() {
		rec.ProcessedAt = time.Now().UTC()
	}
	_, err := db.SQL.ExecContext(ctx, `
		INSERT INTO email_processed (gmail_message_id, application_id, processed_at, classification)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(gmail_message_id) DO UPDATE SET
			application_id = excluded.application_id,
			processed_at = excluded.processed_at,
			classification = excluded.classification`,
		rec.GmailMessageID,
		rec.ApplicationID,
		rec.ProcessedAt.UTC().Format(timeLayout),
		rec.Classification,
	)
	if err != nil {
		return fmt.Errorf("mark email processed: %w", err)
	}
	return nil
}

// IsEmailProcessed reports whether a Gmail message was already handled.
func (db *DB) IsEmailProcessed(ctx context.Context, gmailMessageID string) (bool, error) {
	var one int
	err := db.SQL.QueryRowContext(ctx,
		`SELECT 1 FROM email_processed WHERE gmail_message_id = ?`, gmailMessageID,
	).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// CreateSyncRun inserts a new sync run.
func (db *DB) CreateSyncRun(ctx context.Context, run *domain.SyncRun) error {
	if run.ID == "" {
		run.ID = uuid.NewString()
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = time.Now().UTC()
	}
	_, err := db.SQL.ExecContext(ctx, `
		INSERT INTO sync_runs (
			id, started_at, finished_at, status,
			emails_seen, emails_updated, errors,
			gemini_calls, gemini_skipped_prefilter,
			watermark, error_summary
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID,
		run.StartedAt.UTC().Format(timeLayout),
		formatTime(run.FinishedAt),
		run.Status,
		run.EmailsSeen,
		run.EmailsUpdated,
		run.Errors,
		run.GeminiCalls,
		run.GeminiSkippedPrefilter,
		run.Watermark,
		run.ErrorSummary,
	)
	if err != nil {
		return fmt.Errorf("create sync run: %w", err)
	}
	return nil
}

// FinishSyncRun updates counters/status at the end of a sync.
func (db *DB) FinishSyncRun(ctx context.Context, run *domain.SyncRun) error {
	now := time.Now().UTC()
	run.FinishedAt = &now
	_, err := db.SQL.ExecContext(ctx, `
		UPDATE sync_runs SET
			finished_at = ?, status = ?,
			emails_seen = ?, emails_updated = ?, errors = ?,
			gemini_calls = ?, gemini_skipped_prefilter = ?,
			watermark = ?, error_summary = ?
		WHERE id = ?`,
		run.FinishedAt.Format(timeLayout),
		run.Status,
		run.EmailsSeen,
		run.EmailsUpdated,
		run.Errors,
		run.GeminiCalls,
		run.GeminiSkippedPrefilter,
		run.Watermark,
		run.ErrorSummary,
		run.ID,
	)
	if err != nil {
		return fmt.Errorf("finish sync run: %w", err)
	}
	return nil
}

// GetLastSuccessfulWatermark returns the watermark from the latest successful/partial run.
func (db *DB) GetLastSuccessfulWatermark(ctx context.Context) (string, error) {
	var watermark string
	err := db.SQL.QueryRowContext(ctx, `
		SELECT watermark FROM sync_runs
		WHERE status IN (?, ?) AND watermark != ''
		ORDER BY started_at DESC
		LIMIT 1`,
		domain.SyncStatusSuccess, domain.SyncStatusPartial,
	).Scan(&watermark)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return watermark, nil
}

// GetLastSyncRun returns the most recently finished sync run, if any.
func (db *DB) GetLastSyncRun(ctx context.Context) (*domain.SyncRun, error) {
	row := db.SQL.QueryRowContext(ctx, `
		SELECT id, started_at, finished_at, status,
			emails_seen, emails_updated, errors,
			gemini_calls, gemini_skipped_prefilter,
			watermark, error_summary
		FROM sync_runs
		WHERE finished_at IS NOT NULL AND finished_at != ''
		ORDER BY finished_at DESC
		LIMIT 1`)
	return scanSyncRun(row)
}

func scanSyncRun(row scannable) (*domain.SyncRun, error) {
	var (
		run                                domain.SyncRun
		startedAt, finishedAt              sql.NullString
		watermark, errorSummary            sql.NullString
	)
	err := row.Scan(
		&run.ID, &startedAt, &finishedAt, &run.Status,
		&run.EmailsSeen, &run.EmailsUpdated, &run.Errors,
		&run.GeminiCalls, &run.GeminiSkippedPrefilter,
		&watermark, &errorSummary,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan sync run: %w", err)
	}
	if startedAt.Valid {
		run.StartedAt, err = time.Parse(timeLayout, startedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse started_at: %w", err)
		}
	}
	if finishedAt.Valid && strings.TrimSpace(finishedAt.String) != "" {
		t, err := time.Parse(timeLayout, finishedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse finished_at: %w", err)
		}
		run.FinishedAt = &t
	}
	if watermark.Valid {
		run.Watermark = watermark.String
	}
	if errorSummary.Valid {
		run.ErrorSummary = errorSummary.String
	}
	return &run, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanApplication(row scannable) (*domain.Application, error) {
	var (
		app                                domain.Application
		applied, interview, oa             sql.NullString
		sourceEmailID, sheetRowID          sql.NullString
		createdAt, updatedAt               string
	)
	err := row.Scan(
		&app.ID, &app.Company, &app.Position, &app.Status,
		&applied, &interview, &oa,
		&sourceEmailID, &sheetRowID, &app.RawExcerpt,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan application: %w", err)
	}

	app.AppliedAt, err = parseTime(applied)
	if err != nil {
		return nil, err
	}
	app.InterviewAt, err = parseTime(interview)
	if err != nil {
		return nil, err
	}
	app.OAAt, err = parseTime(oa)
	if err != nil {
		return nil, err
	}
	if sourceEmailID.Valid {
		app.SourceEmailID = sourceEmailID.String
	}
	if sheetRowID.Valid {
		app.SheetRowID = sheetRowID.String
	}
	app.CreatedAt, err = time.Parse(timeLayout, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	app.UpdatedAt, err = time.Parse(timeLayout, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}
	return &app, nil
}

func formatTime(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.UTC().Format(timeLayout)
}

func parseTime(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid || strings.TrimSpace(ns.String) == "" {
		return nil, nil
	}
	t, err := time.Parse(timeLayout, ns.String)
	if err != nil {
		return nil, fmt.Errorf("parse time %q: %w", ns.String, err)
	}
	return &t, nil
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
