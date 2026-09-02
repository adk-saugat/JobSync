package sheets

import (
	"testing"

	"google.golang.org/api/googleapi"
)

func TestHeaderMatches(t *testing.T) {
	row := make([]any, len(Headers))
	for i, h := range Headers {
		row[i] = h
	}
	if !headerMatches(row) {
		t.Fatal("expected headers to match")
	}
	row[1] = "Nope"
	if headerMatches(row) {
		t.Fatal("expected mismatch")
	}
}

func TestRowValuesOrder(t *testing.T) {
	vals := rowValues(Row{
		RowID:       "id-1",
		Company:     "Acme",
		Position:    "SWE",
		Status:      "applied",
		AppliedAt:   "2026-01-01",
		InterviewAt: "",
		OAAt:        "",
		Notes:       "hi",
	})
	if len(vals) != len(Headers) {
		t.Fatalf("len=%d want %d", len(vals), len(Headers))
	}
	if vals[0] != "id-1" || vals[1] != "Acme" || vals[3] != "applied" {
		t.Fatalf("unexpected values: %#v", vals)
	}
}

func TestVisibleHeaders(t *testing.T) {
	if len(Headers) != len(VisibleHeaders)+1 {
		t.Fatalf("headers=%d visible=%d", len(Headers), len(VisibleHeaders))
	}
	if Headers[0] != "Row ID" {
		t.Fatalf("first header = %q", Headers[0])
	}
}

func TestIndexByCompanyPosition(t *testing.T) {
	values := [][]any{
		{"Row ID", "Company", "Position"},
		{"id-1", "Serval", "Software Engineer Intern"},
		{"id-2", "Zipline", "Software Engineer Intern (Spring 2027)"},
		{"id-3", "serval", "software engineer intern"}, // duplicate, later
	}
	idx := indexByCompanyPosition(values, "Serval", "Software Engineer Intern")
	if idx != 1 {
		t.Fatalf("idx=%d want 1 (first match)", idx)
	}
	if indexByCompanyPosition(values, "Missing", "Role") != -1 {
		t.Fatal("expected miss")
	}
	if indexByRowID(values, "id-2") != 2 {
		t.Fatal("expected id-2 at index 2")
	}
	if nextEmptySheetRow(values) != 5 {
		t.Fatalf("next empty = %d want 5", nextEmptySheetRow(values))
	}
}

func TestIsTransientGoogleAPI(t *testing.T) {
	if !isTransientGoogleAPI(&googleapi.Error{Code: 503, Message: "unavailable"}) {
		t.Fatal("expected 503 transient")
	}
	if isTransientGoogleAPI(&googleapi.Error{Code: 403, Message: "forbidden"}) {
		t.Fatal("403 should not be transient")
	}
}
