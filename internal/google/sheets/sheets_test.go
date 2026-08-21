package sheets

import "testing"

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
