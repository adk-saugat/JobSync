package storage_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/saugatadhikari/jobSync/internal/storage"
	"github.com/saugatadhikari/jobSync/internal/domain"
)

func TestCreateAndGetApplication(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")

	database, err := storage.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = storage.Close(database) })

	applied := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	app := &domain.Application{
		Company:       "Acme Corp",
		Position:      "Software Engineer",
		Status:        domain.StatusApplied,
		AppliedAt:     &applied,
		SourceEmailID: "gmail-msg-1",
		SheetRowID:    "row-uuid-1",
		RawExcerpt:    "Thanks for applying",
	}
	if err := database.CreateApplication(ctx, app); err != nil {
		t.Fatalf("create: %v", err)
	}
	if app.ID == "" {
		t.Fatal("expected id to be set")
	}

	got, err := database.GetApplicationByID(ctx, app.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("expected application")
	}
	if got.Company != "Acme Corp" || got.Position != "Software Engineer" {
		t.Fatalf("unexpected app: %+v", got)
	}
	if got.Status != domain.StatusApplied {
		t.Fatalf("status = %q", got.Status)
	}
	if got.SourceEmailID != "gmail-msg-1" {
		t.Fatalf("source email = %q", got.SourceEmailID)
	}

	byEmail, err := database.FindBySourceEmailID(ctx, "gmail-msg-1")
	if err != nil || byEmail == nil || byEmail.ID != app.ID {
		t.Fatalf("FindBySourceEmailID: got=%v err=%v", byEmail, err)
	}

	byCompany, err := database.FindByCompanyAndPosition(ctx, "acme corp", "software engineer")
	if err != nil || byCompany == nil || byCompany.ID != app.ID {
		t.Fatalf("FindByCompanyAndPosition: got=%v err=%v", byCompany, err)
	}
}

func TestEmailProcessedAndSyncRun(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")

	database, err := storage.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = storage.Close(database) })

	app := &domain.Application{
		Company:  "Beta",
		Position: "Intern",
		Status:   domain.StatusAssessment,
	}
	if err := database.CreateApplication(ctx, app); err != nil {
		t.Fatalf("create app: %v", err)
	}

	processed, err := database.IsEmailProcessed(ctx, "msg-2")
	if err != nil || processed {
		t.Fatalf("expected not processed, got %v err=%v", processed, err)
	}

	appID := app.ID
	if err := database.MarkEmailProcessed(ctx, domain.EmailProcessed{
		GmailMessageID: "msg-2",
		ApplicationID:  &appID,
		Classification: domain.ClassificationJobUpdate,
	}); err != nil {
		t.Fatalf("mark: %v", err)
	}

	processed, err = database.IsEmailProcessed(ctx, "msg-2")
	if err != nil || !processed {
		t.Fatalf("expected processed, got %v err=%v", processed, err)
	}

	run := &domain.SyncRun{
		Status:    domain.SyncStatusSuccess,
		Watermark: "wm-100",
	}
	if err := database.CreateSyncRun(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	run.EmailsSeen = 3
	run.EmailsUpdated = 1
	run.GeminiCalls = 2
	if err := database.FinishSyncRun(ctx, run); err != nil {
		t.Fatalf("finish run: %v", err)
	}

	wm, err := database.GetLastSuccessfulWatermark(ctx)
	if err != nil {
		t.Fatalf("watermark: %v", err)
	}
	if wm != "wm-100" {
		t.Fatalf("watermark = %q", wm)
	}
}
