// Package domain holds core JobSync types and status constants.
package domain

import "time"

// Application status values written to Google Sheets.
const (
	StatusApplied    = "applied"
	StatusAssessment = "assessment" // online assessment / coding challenge
	StatusInterview  = "interview"
	StatusRejected   = "rejected"
	StatusAccepted   = "accepted" // offer accepted / offer received
	StatusOther      = "other"

	// Deprecated aliases (still recognized when reading old data / model output).
	StatusOA    = StatusAssessment
	StatusOffer = StatusAccepted
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

// EmailProcessed records that a Gmail message was handled (or ignored).
type EmailProcessed struct {
	GmailMessageID string
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

// Account holds per-user cloud settings and secrets for server-side sync.
type Account struct {
	ID                string
	SpreadsheetID     string
	SheetName         string
	GeminiAPIKey      string
	GeminiModel       string
	OAuthTokenJSON    string
	AuthScopesVersion int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (a *Account) HasGeminiKey() bool {
	return a != nil && a.GeminiAPIKey != ""
}

func (a *Account) HasSpreadsheet() bool {
	return a != nil && a.SpreadsheetID != ""
}

func (a *Account) HasOAuthToken() bool {
	return a != nil && a.OAuthTokenJSON != ""
}

func StatusRank(status string) int {
	switch status {
	case StatusApplied:
		return 1
	case StatusAssessment:
		return 2
	case StatusInterview:
		return 3
	case StatusAccepted:
		return 4
	default:
		return 0
	}
}

func ShouldUpdateStatus(current, next string) bool {
	if next == "" || next == current {
		return false
	}
	if next == StatusRejected {
		return current != StatusAccepted
	}
	if current == StatusRejected {
		return next == StatusAccepted
	}
	return StatusRank(next) > StatusRank(current)
}
