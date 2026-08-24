package syncer

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/saugatadhikari/jobSync/internal/domain"
	"github.com/saugatadhikari/jobSync/internal/gemini"
	"github.com/saugatadhikari/jobSync/internal/google/gmail"
	"github.com/saugatadhikari/jobSync/internal/google/sheets"
)

type fakeGmail struct {
	metas []gmail.MessageMeta
	msgs  map[string]*gmail.Message
}

func (f *fakeGmail) Search(ctx context.Context, query string, limit int64, after time.Time) ([]gmail.MessageMeta, error) {
	return f.metas, nil
}
func (f *fakeGmail) GetMessage(ctx context.Context, id string) (*gmail.Message, error) {
	m, ok := f.msgs[id]
	if !ok {
		return nil, errors.New("missing")
	}
	return m, nil
}

type fakeGemini struct {
	ext   *gemini.Extraction
	err   error
	calls int
}

func (f *fakeGemini) Extract(ctx context.Context, in gemini.EmailInput) (*gemini.Extraction, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.ext, nil
}
func (f *fakeGemini) MinConfidence() float64 { return 0.6 }

type fakeSheets struct {
	appended int
	updated  int
	written  int
	rows     map[string]sheets.Row // company|position lower -> row
}

func (f *fakeSheets) key(c, p string) string {
	return strings.ToLower(c) + "|" + strings.ToLower(p)
}

func (f *fakeSheets) FindByCompanyAndPosition(ctx context.Context, company, position string) (*sheets.Row, error) {
	if f.rows == nil {
		return nil, nil
	}
	if row, ok := f.rows[f.key(company, position)]; ok {
		cp := row
		return &cp, nil
	}
	return nil, nil
}

func (f *fakeSheets) WriteRow(ctx context.Context, row sheets.Row) error {
	f.written++
	if f.rows == nil {
		f.rows = map[string]sheets.Row{}
	}
	k := f.key(row.Company, row.Position)
	if _, ok := f.rows[k]; ok {
		f.updated++
	} else {
		f.appended++
	}
	f.rows[k] = row
	return nil
}

type fakeStore struct {
	processed map[string]bool
	apps      map[string]*domain.Application // key company|position lower
	runs      int
}

func newFakeStore() *fakeStore {
	return &fakeStore{processed: map[string]bool{}, apps: map[string]*domain.Application{}}
}

func (f *fakeStore) key(c, p string) string { return c + "|" + p }

func (f *fakeStore) IsEmailProcessed(ctx context.Context, id string) (bool, error) {
	return f.processed[id], nil
}
func (f *fakeStore) MarkEmailProcessed(ctx context.Context, rec domain.EmailProcessed) error {
	f.processed[rec.GmailMessageID] = true
	return nil
}
func (f *fakeStore) FindByCompanyAndPosition(ctx context.Context, company, position string) (*domain.Application, error) {
	if app, ok := f.apps[f.key(company, position)]; ok {
		cp := *app
		return &cp, nil
	}
	return nil, nil
}
func (f *fakeStore) CreateApplication(ctx context.Context, app *domain.Application) error {
	if app.ID == "" {
		app.ID = "app-1"
	}
	cp := *app
	f.apps[f.key(app.Company, app.Position)] = &cp
	return nil
}
func (f *fakeStore) UpdateApplication(ctx context.Context, app *domain.Application) error {
	cp := *app
	f.apps[f.key(app.Company, app.Position)] = &cp
	return nil
}
func (f *fakeStore) GetLastSuccessfulWatermark(ctx context.Context) (string, error) {
	return "", nil
}
func (f *fakeStore) CreateSyncRun(ctx context.Context, run *domain.SyncRun) error {
	f.runs++
	run.ID = "run-1"
	return nil
}
func (f *fakeStore) FinishSyncRun(ctx context.Context, run *domain.SyncRun) error { return nil }

func TestRunCreatesApplication(t *testing.T) {
	date := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	store := newFakeStore()
	sheetsAPI := &fakeSheets{}
	gem := &fakeGemini{ext: &gemini.Extraction{
		IsJobRelated: true,
		Company:      "Acme",
		Position:     "SWE",
		Status:       domain.StatusInterview,
		Confidence:   0.9,
		Summary:      "Interview invite",
	}}
	g := &fakeGmail{
		metas: []gmail.MessageMeta{{ID: "m1", Date: date}},
		msgs: map[string]*gmail.Message{
			"m1": {ID: "m1", Subject: "Interview invitation", From: "hr@acme.com", Date: date, Body: "Please interview"},
		},
	}

	runner := &Runner{Gmail: g, Gemini: gem, Sheets: sheetsAPI, DB: store}
	res, err := runner.Run(context.Background(), Options{Limit: 5, MinInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if res.EmailsCreated != 1 || gem.calls != 1 || sheetsAPI.appended != 1 {
		t.Fatalf("res=%+v geminiCalls=%d appended=%d", res, gem.calls, sheetsAPI.appended)
	}
	if !store.processed["m1"] {
		t.Fatal("expected processed")
	}

	// Second run should skip Gemini.
	gem.calls = 0
	res2, err := runner.Run(context.Background(), Options{Limit: 5, MinInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if gem.calls != 0 || res2.EmailsSkippedProcessed != 1 {
		t.Fatalf("expected skip, got calls=%d skipped=%d", gem.calls, res2.EmailsSkippedProcessed)
	}
}

func TestRunDryRunNoWrites(t *testing.T) {
	date := time.Now().UTC()
	store := newFakeStore()
	sheetsAPI := &fakeSheets{}
	gem := &fakeGemini{ext: &gemini.Extraction{
		IsJobRelated: true, Company: "Beta", Position: "Intern",
		Status: domain.StatusApplied, Confidence: 0.95, Summary: "Thanks",
	}}
	g := &fakeGmail{
		metas: []gmail.MessageMeta{{ID: "m2", Date: date}},
		msgs: map[string]*gmail.Message{
			"m2": {ID: "m2", Subject: "Thanks for applying", From: "jobs@beta.com", Date: date, Body: "We received"},
		},
	}
	runner := &Runner{Gmail: g, Gemini: gem, Sheets: sheetsAPI, DB: store}
	res, err := runner.Run(context.Background(), Options{Limit: 5, DryRun: true, MinInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if res.EmailsCreated != 1 {
		t.Fatalf("expected would-create, got %+v", res)
	}
	if sheetsAPI.appended != 0 || len(store.apps) != 0 || store.processed["m2"] {
		t.Fatal("dry-run must not write")
	}
}

func TestRunReclaimsExistingSheetRow(t *testing.T) {
	date := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	store := newFakeStore()
	sheetsAPI := &fakeSheets{
		rows: map[string]sheets.Row{
			"serval|software engineer intern": {
				RowID:    "sheet-row-1",
				Company:  "Serval",
				Position: "Software Engineer Intern",
				Status:   domain.StatusApplied,
			},
		},
	}
	gem := &fakeGemini{ext: &gemini.Extraction{
		IsJobRelated: true,
		Company:      "Serval",
		Position:     "Software Engineer Intern",
		Status:       domain.StatusApplied,
		Confidence:   0.9,
		Summary:      "Thanks for applying",
	}}
	g := &fakeGmail{
		metas: []gmail.MessageMeta{{ID: "m3", Date: date}},
		msgs: map[string]*gmail.Message{
			"m3": {ID: "m3", Subject: "Thanks for applying to Serval!", From: "jobs@serval.com", Date: date, Body: "We received"},
		},
	}

	runner := &Runner{Gmail: g, Gemini: gem, Sheets: sheetsAPI, DB: store}
	res, err := runner.Run(context.Background(), Options{Limit: 5, MinInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if res.EmailsCreated != 0 || res.EmailsUpdated != 1 {
		t.Fatalf("expected reclaim update, got %+v", res)
	}
	if sheetsAPI.appended != 0 || sheetsAPI.updated != 1 {
		t.Fatalf("expected update not append: appended=%d updated=%d", sheetsAPI.appended, sheetsAPI.updated)
	}
	app, err := store.FindByCompanyAndPosition(context.Background(), "Serval", "Software Engineer Intern")
	if err != nil || app == nil {
		t.Fatalf("expected app in db: %v", err)
	}
	if app.SheetRowID != "sheet-row-1" {
		t.Fatalf("sheet row id = %q", app.SheetRowID)
	}
}

func TestParseFlexibleTime(t *testing.T) {
	s := "2026-08-01"
	got := parseFlexibleTime(&s)
	if got == nil || got.Format("2006-01-02") != "2026-08-01" {
		t.Fatalf("got %v", got)
	}
}
