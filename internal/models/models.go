package models

import "time"

// Application status values written to SQLite and Google Sheets.
const (
	StatusApplied   = "applied"
	StatusOA        = "oa"
	StatusInterview = "interview"
	StatusRejected  = "rejected"
	StatusOffer     = "offer"
	StatusOther     = "other"
)

// EmailClassification values for email_processed.
const (
	ClassificationJobUpdate = "job_update"
	ClassificationIgnored   = "ignored"
	ClassificationError     = "error"
)

// SyncRun status values.
const (
	SyncStatusSuccess        = "success"
	SyncStatusPartial        = "partial"
	SyncStatusFailed         = "failed"
	SyncStatusQuotaExhausted = "quota_exhausted"
)

// Application is one tracked job application.
type Application struct {
	ID            string
	Company       string
	Position      string
	Status        string
	AppliedAt     *time.Time
	InterviewAt   *time.Time
	OAAt          *time.Time
	SourceEmailID string
	SheetRowID    string
	RawExcerpt    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// EmailProcessed records that a Gmail message was handled (or ignored).
type EmailProcessed struct {
	GmailMessageID string
	ApplicationID  *string
	ProcessedAt    time.Time
	Classification string
}

// SyncRun is one jobsync sync execution (manual or scheduled).
type SyncRun struct {
	ID                     string
	StartedAt              time.Time
	FinishedAt             *time.Time
	Status                 string
	EmailsSeen             int
	EmailsUpdated          int
	Errors                 int
	GeminiCalls            int
	GeminiSkippedPrefilter int
	Watermark              string
	ErrorSummary           string
}
