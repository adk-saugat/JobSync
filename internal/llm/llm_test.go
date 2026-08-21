package llm

import (
	"errors"
	"strings"
	"testing"
)

func TestParseExtraction(t *testing.T) {
	raw := `{
	  "is_job_related": true,
	  "company": "Acme",
	  "position": "SWE",
	  "status": "interview",
	  "applied_at": "2026-08-01",
	  "interview_at": null,
	  "oa_at": null,
	  "confidence": 0.9,
	  "summary": "Interview invite",
	  "ignore_reason": null
	}`
	ext, err := ParseExtraction(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !ext.IsJobRelated || ext.Company != "Acme" || ext.Status != "interview" {
		t.Fatalf("%+v", ext)
	}
}

func TestParseExtractionMarkdownFence(t *testing.T) {
	raw := "```json\n{\"is_job_related\":false,\"company\":\"\",\"position\":\"\",\"status\":\"other\",\"confidence\":0.2,\"summary\":\"\",\"ignore_reason\":\"newsletter\"}\n```"
	ext, err := ParseExtraction(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ext.IsJobRelated {
		t.Fatal("expected not job related")
	}
}

func TestTruncate(t *testing.T) {
	got := Truncate("abcdefghij", 5)
	if !strings.HasPrefix(got, "abcde") || !strings.HasSuffix(got, "…") {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeUnknownStatus(t *testing.T) {
	ext := &Extraction{IsJobRelated: true, Status: "PHONE_SCREEN"}
	normalizeExtraction(ext)
	if ext.Status != "other" {
		t.Fatalf("status=%q", ext.Status)
	}
}

func TestLooksLikeQuota(t *testing.T) {
	if !looksLikeQuota(`{"error":{"status":"RESOURCE_EXHAUSTED","message":"Quota exceeded"}}`) {
		t.Fatal("expected quota")
	}
	if errors.Is(ErrQuotaExceeded, ErrQuotaExceeded) != true {
		t.Fatal("sentinel")
	}
}
