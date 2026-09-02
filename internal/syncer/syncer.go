package syncer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/saugatadhikari/jobSync/internal/domain"
	"github.com/saugatadhikari/jobSync/internal/gemini"
	"github.com/saugatadhikari/jobSync/internal/google/gmail"
	"github.com/saugatadhikari/jobSync/internal/google/sheets"
)

// Options configures one sync run.
type Options struct {
	Limit      int64         // max Gemini calls (default 15)
	DryRun     bool          // no SQLite / Sheets writes
	MinInterval time.Duration // pause between Gemini calls
	Query      string
	// Since overrides the stored watermark, so mail already behind it is
	// scanned again. Used by `jobsync sync --since` to recover missed mail.
	Since time.Time
}

// Result summarizes a sync run.
type Result struct {
	EmailsSeen             int
	EmailsCreated          int
	EmailsUpdated          int
	EmailsIgnored          int
	EmailsSkippedProcessed int
	Errors                 int
	GeminiCalls            int
	GeminiSkippedPrefilter int
	QuotaExhausted         bool
	DryRun                 bool
	Watermark              string
	Status                 string
	ErrorSummary           string
}

// GmailAPI is the Gmail surface used by syncer.
type GmailAPI interface {
	Search(ctx context.Context, query string, limit int64, after time.Time) ([]gmail.MessageMeta, error)
	GetMessage(ctx context.Context, id string) (*gmail.Message, error)
}

// GeminiAPI is the extraction surface used by syncer.
type GeminiAPI interface {
	Extract(ctx context.Context, in gemini.EmailInput) (*gemini.Extraction, error)
	MinConfidence() float64
}

// SheetsAPI is the tracker surface used by syncer.
type SheetsAPI interface {
	WriteRow(ctx context.Context, row sheets.Row) error
	FindByCompanyAndPosition(ctx context.Context, company, position string) (*sheets.Row, error)
}

// Store is the SQLite surface used by syncer.
type Store interface {
	IsEmailProcessed(ctx context.Context, gmailMessageID string) (bool, error)
	MarkEmailProcessed(ctx context.Context, rec domain.EmailProcessed) error
	FindByCompanyAndPosition(ctx context.Context, company, position string) (*domain.Application, error)
	CreateApplication(ctx context.Context, app *domain.Application) error
	UpdateApplication(ctx context.Context, app *domain.Application) error
	GetLastSuccessfulWatermark(ctx context.Context) (string, error)
	CreateSyncRun(ctx context.Context, run *domain.SyncRun) error
	FinishSyncRun(ctx context.Context, run *domain.SyncRun) error
}

// Runner orchestrates Gmail → Gemini → SQLite → Sheets.
type Runner struct {
	Gmail  GmailAPI
	Gemini GeminiAPI
	Sheets SheetsAPI
	DB     Store
	Log    func(format string, args ...any)
}

func (r *Runner) logf(format string, args ...any) {
	if r.Log != nil {
		r.Log(format, args...)
	}
}

// Run executes one sync.
func (r *Runner) Run(ctx context.Context, opts Options) (*Result, error) {
	if opts.Limit <= 0 {
		opts.Limit = 15
	}
	if opts.MinInterval <= 0 {
		opts.MinInterval = 500 * time.Millisecond
	}
	if opts.Query == "" {
		opts.Query = gmail.DefaultQuery
	}

	res := &Result{DryRun: opts.DryRun, Status: domain.SyncStatusSuccess}
	run := &domain.SyncRun{
		Status:    domain.SyncStatusSuccess,
		StartedAt: time.Now().UTC(),
	}

	if !opts.DryRun {
		if err := r.DB.CreateSyncRun(ctx, run); err != nil {
			return nil, err
		}
	}

	watermarkStr, err := r.DB.GetLastSuccessfulWatermark(ctx)
	if err != nil {
		return nil, err
	}
	var after time.Time
	if watermarkStr != "" {
		if t, err := time.Parse(time.RFC3339, watermarkStr); err == nil {
			after = t
		}
	}
	if !opts.Since.IsZero() {
		after = opts.Since
		watermarkStr = ""
	}

	searchLimit := opts.Limit * 3
	if searchLimit < 20 {
		searchLimit = 20
	}

	metas, err := r.Gmail.Search(ctx, opts.Query, searchLimit, after)
	if err != nil {
		res.Status = domain.SyncStatusFailed
		res.ErrorSummary = err.Error()
		r.finish(ctx, run, res, opts.DryRun)
		return res, err
	}
	res.EmailsSeen = len(metas)

	var errNotes []string

	// Gmail returns newest-first, so the watermark may only advance past mail
	// once everything older has been dealt with. Advancing on the newest
	// handled message alone permanently strands anything skipped below it.
	var newestDone, oldestPending time.Time
	markDone := func(t time.Time) {
		if t.After(newestDone) {
			newestDone = t
		}
	}
	markPending := func(t time.Time) {
		if oldestPending.IsZero() || t.Before(oldestPending) {
			oldestPending = t
		}
	}

	for _, meta := range metas {
		if int64(res.GeminiCalls) >= opts.Limit {
			markPending(meta.Date)
			continue
		}

		processed, err := r.DB.IsEmailProcessed(ctx, meta.ID)
		if err != nil {
			res.Errors++
			errNotes = append(errNotes, err.Error())
			markPending(meta.Date)
			continue
		}
		if processed {
			res.EmailsSkippedProcessed++
			markDone(meta.Date)
			continue
		}

		msg, err := r.Gmail.GetMessage(ctx, meta.ID)
		if err != nil {
			res.Errors++
			errNotes = append(errNotes, err.Error())
			markPending(meta.Date)
			continue
		}

		if !gmail.LooksLikeStatusUpdate(msg.Subject, msg.From) {
			// Not persisted and not watermarked: a tighter prefilter can
			// otherwise permanently miss real OA mail (e.g. "Assessment for…").
			res.GeminiSkippedPrefilter++
			markPending(msg.Date)
			continue
		}

		r.logf("Extracting: %s", msg.Subject)
		ext, err := r.Gemini.Extract(ctx, gemini.EmailInput{
			Subject: msg.Subject,
			From:    msg.From,
			Date:    msg.Date,
			Body:    msg.Body,
		})
		res.GeminiCalls++
		if err != nil {
			if errors.Is(err, gemini.ErrQuotaExceeded) {
				res.QuotaExhausted = true
				res.Status = domain.SyncStatusQuotaExhausted
				errNotes = append(errNotes, "gemini quota exceeded")
				r.logf("Gemini quota exceeded — stopping")
				markPending(msg.Date)
				break
			}
			res.Errors++
			errNotes = append(errNotes, err.Error())
			if !opts.DryRun {
				_ = r.DB.MarkEmailProcessed(ctx, domain.EmailProcessed{
					GmailMessageID: msg.ID,
					Classification: domain.ClassificationError,
				})
			}
			markDone(msg.Date)
			continue
		}

		if !ext.IsJobRelated || ext.Confidence < r.Gemini.MinConfidence() {
			res.EmailsIgnored++
			reason := ext.IgnoreReason
			if reason == "" && !ext.IsJobRelated {
				reason = "not a status update"
			}
			if reason == "" {
				reason = "low confidence"
			}
			r.logf("Ignored: %s (%s)", msg.Subject, reason)
			if !opts.DryRun {
				_ = r.DB.MarkEmailProcessed(ctx, domain.EmailProcessed{
					GmailMessageID: msg.ID,
					Classification: domain.ClassificationIgnored,
				})
			}
			markDone(msg.Date)
			time.Sleep(opts.MinInterval)
			continue
		}

		created, updated, appID, err := r.upsert(ctx, msg, ext, opts.DryRun)
		if err != nil {
			res.Errors++
			errNotes = append(errNotes, err.Error())
			r.logf("Upsert error: %v", err)
			if !opts.DryRun {
				_ = r.DB.MarkEmailProcessed(ctx, domain.EmailProcessed{
					GmailMessageID: msg.ID,
					Classification: domain.ClassificationError,
				})
			}
			markDone(msg.Date)
			time.Sleep(opts.MinInterval)
			continue
		}
		if created {
			res.EmailsCreated++
			r.logf("Created: %s / %s [%s]", ext.Company, ext.Position, ext.Status)
		} else if updated {
			res.EmailsUpdated++
			r.logf("Updated: %s / %s [%s]", ext.Company, ext.Position, ext.Status)
		} else {
			r.logf("No change: %s / %s", ext.Company, ext.Position)
		}

		if !opts.DryRun {
			_ = r.DB.MarkEmailProcessed(ctx, domain.EmailProcessed{
				GmailMessageID: msg.ID,
				ApplicationID:  &appID,
				Classification: domain.ClassificationJobUpdate,
			})
		}
		markDone(msg.Date)
		time.Sleep(opts.MinInterval)
	}

	res.Watermark = watermarkStr
	if !newestDone.IsZero() && (oldestPending.IsZero() || newestDone.Before(oldestPending)) {
		res.Watermark = newestDone.UTC().Format(time.RFC3339)
	}

	if res.QuotaExhausted {
		res.Status = domain.SyncStatusQuotaExhausted
	} else if res.Errors > 0 {
		res.Status = domain.SyncStatusPartial
	}
	if len(errNotes) > 0 {
		res.ErrorSummary = strings.Join(errNotes, "; ")
		if len(res.ErrorSummary) > 500 {
			res.ErrorSummary = res.ErrorSummary[:500] + "…"
		}
	}

	r.finish(ctx, run, res, opts.DryRun)
	return res, nil
}

func (r *Runner) finish(ctx context.Context, run *domain.SyncRun, res *Result, dryRun bool) {
	run.Status = res.Status
	run.EmailsSeen = res.EmailsSeen
	run.EmailsUpdated = res.EmailsCreated + res.EmailsUpdated
	run.Errors = res.Errors
	run.GeminiCalls = res.GeminiCalls
	run.GeminiSkippedPrefilter = res.GeminiSkippedPrefilter
	run.Watermark = res.Watermark
	run.ErrorSummary = res.ErrorSummary
	if dryRun {
		return
	}
	_ = r.DB.FinishSyncRun(ctx, run)
}

func (r *Runner) upsert(ctx context.Context, msg *gmail.Message, ext *gemini.Extraction, dryRun bool) (created, updated bool, appID string, err error) {
	company := strings.TrimSpace(ext.Company)
	position := strings.TrimSpace(ext.Position)
	if company == "" || position == "" {
		return false, false, "", fmt.Errorf("missing company/position")
	}

	existing, err := r.DB.FindByCompanyAndPosition(ctx, company, position)
	if err != nil {
		return false, false, "", err
	}

	appliedAt := parseFlexibleTime(ext.AppliedAt)
	interviewAt := parseFlexibleTime(ext.InterviewAt)
	oaAt := parseFlexibleTime(ext.OAAt)
	excerpt := ext.Summary
	if excerpt == "" {
		excerpt = msg.Snippet
	}
	if len(excerpt) > 280 {
		excerpt = excerpt[:280]
	}

	if existing == nil {
		// Sheet is shared across local + cloud DBs. Reclaim an existing sheet row
		// so we update instead of appending a duplicate.
		sheetRow, err := r.Sheets.FindByCompanyAndPosition(ctx, company, position)
		if err != nil {
			return false, false, "", err
		}
		rowID := uuid.NewString()
		status := ext.Status
		if sheetRow != nil {
			if strings.TrimSpace(sheetRow.RowID) != "" {
				rowID = strings.TrimSpace(sheetRow.RowID)
			}
			if !domain.ShouldUpdateStatus(sheetRow.Status, ext.Status) && sheetRow.Status != "" {
				status = sheetRow.Status
			}
		}
		app := &domain.Application{
			Company:       company,
			Position:      position,
			Status:        status,
			AppliedAt:     appliedAt,
			InterviewAt:   interviewAt,
			OAAt:          oaAt,
			SourceEmailID: msg.ID,
			SheetRowID:    rowID,
			RawExcerpt:    excerpt,
		}
		if dryRun {
			if sheetRow != nil {
				return false, true, rowID, nil
			}
			return true, false, rowID, nil
		}
		if err := r.DB.CreateApplication(ctx, app); err != nil {
			return false, false, "", err
		}
		if err := r.Sheets.WriteRow(ctx, toSheetRow(app)); err != nil {
			return false, false, "", err
		}
		if sheetRow != nil {
			return false, true, app.ID, nil
		}
		return true, false, app.ID, nil
	}

	changed := false
	if domain.ShouldUpdateStatus(existing.Status, ext.Status) {
		existing.Status = ext.Status
		changed = true
	}
	if existing.AppliedAt == nil && appliedAt != nil {
		existing.AppliedAt = appliedAt
		changed = true
	}
	if existing.InterviewAt == nil && interviewAt != nil {
		existing.InterviewAt = interviewAt
		changed = true
	}
	if existing.OAAt == nil && oaAt != nil {
		existing.OAAt = oaAt
		changed = true
	}
	if existing.SourceEmailID == "" {
		existing.SourceEmailID = msg.ID
		changed = true
	}
	if excerpt != "" && existing.RawExcerpt != excerpt {
		existing.RawExcerpt = excerpt
		changed = true
	}
	if existing.SheetRowID == "" {
		existing.SheetRowID = uuid.NewString()
		changed = true
	}

	if !changed {
		return false, false, existing.ID, nil
	}
	if dryRun {
		return false, true, existing.ID, nil
	}
	if err := r.DB.UpdateApplication(ctx, existing); err != nil {
		return false, false, "", err
	}
	if err := r.Sheets.WriteRow(ctx, toSheetRow(existing)); err != nil {
		return false, false, "", err
	}
	return false, true, existing.ID, nil
}

func toSheetRow(app *domain.Application) sheets.Row {
	return sheets.Row{
		RowID:       app.SheetRowID,
		Company:     app.Company,
		Position:    app.Position,
		Status:      app.Status,
		AppliedAt:   formatDate(app.AppliedAt),
		InterviewAt: formatDate(app.InterviewAt),
		OAAt:        formatDate(app.OAAt),
		Notes:       app.RawExcerpt,
	}
}

func formatDate(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

func parseFlexibleTime(s *string) *time.Time {
	if s == nil {
		return nil
	}
	raw := strings.TrimSpace(*s)
	if raw == "" || strings.EqualFold(raw, "null") {
		return nil
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01-02 15:04:05",
		"01/02/2006",
		"January 2, 2006",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			utc := t.UTC()
			return &utc
		}
	}
	return nil
}
