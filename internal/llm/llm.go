package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/saugatadhikari/jobSync/internal/models"
)

const (
	DefaultModel           = "gemini-3.6-flash"
	DefaultBodyCharLimit   = 3000
	DefaultMaxOutputTokens = 512
	DefaultMinConfidence   = 0.6
	apiBase                = "https://generativelanguage.googleapis.com/v1beta"
)

// ErrQuotaExceeded means the AI Studio free-tier limit was hit.
var ErrQuotaExceeded = errors.New("gemini quota exceeded")

// EmailInput is the truncated email content sent to Gemini.
type EmailInput struct {
	Subject string
	From    string
	Date    time.Time
	Body    string
}

// Extraction is the structured result from Gemini.
type Extraction struct {
	IsJobRelated bool    `json:"is_job_related"`
	Company      string  `json:"company"`
	Position     string  `json:"position"`
	Status       string  `json:"status"`
	AppliedAt    *string `json:"applied_at"`
	InterviewAt  *string `json:"interview_at"`
	OAAt         *string `json:"oa_at"`
	Confidence   float64 `json:"confidence"`
	Summary      string  `json:"summary"`
	IgnoreReason string  `json:"ignore_reason"`
}

// Client calls Gemini via Google AI Studio (API key).
type Client struct {
	apiKey         string
	model          string
	bodyCharLimit  int
	maxOutTokens   int
	minConfidence  float64
	httpClient     *http.Client
}

// Options configures the Gemini client.
type Options struct {
	APIKey        string
	Model         string
	BodyCharLimit int
	MaxOutTokens  int
	MinConfidence float64
	HTTPClient    *http.Client
}

// NewClient builds a Gemini client.
func NewClient(opts Options) (*Client, error) {
	if strings.TrimSpace(opts.APIKey) == "" {
		return nil, fmt.Errorf("gemini api key is required (run jobsync init or set gemini_api_key)")
	}
	model := opts.Model
	if model == "" {
		model = DefaultModel
	}
	bodyLimit := opts.BodyCharLimit
	if bodyLimit <= 0 {
		bodyLimit = DefaultBodyCharLimit
	}
	maxOut := opts.MaxOutTokens
	if maxOut <= 0 {
		maxOut = DefaultMaxOutputTokens
	}
	minConf := opts.MinConfidence
	if minConf <= 0 {
		minConf = DefaultMinConfidence
	}
	hc := opts.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 60 * time.Second}
	}
	return &Client{
		apiKey:        opts.APIKey,
		model:         model,
		bodyCharLimit: bodyLimit,
		maxOutTokens:  maxOut,
		minConfidence: minConf,
		httpClient:    hc,
	}, nil
}

// MinConfidence returns the configured confidence threshold.
func (c *Client) MinConfidence() float64 { return c.minConfidence }

// Extract asks Gemini to parse one email into structured fields.
func (c *Client) Extract(ctx context.Context, in EmailInput) (*Extraction, error) {
	body := Truncate(in.Body, c.bodyCharLimit)
	prompt := buildPrompt(in.Subject, in.From, in.Date, body)

	reqBody := generateContentRequest{
		Contents: []content{{
			Parts: []part{{Text: prompt}},
		}},
		GenerationConfig: generationConfig{
			Temperature:      0.2,
			MaxOutputTokens:  c.maxOutTokens,
			ResponseMIMEType: "application/json",
		},
		SystemInstruction: &content{
			Parts: []part{{Text: systemInstruction}},
		},
	}

	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", apiBase, c.model, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 429 {
		return nil, fmt.Errorf("%w: %s", ErrQuotaExceeded, shortErr(respBytes))
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusPaymentRequired {
		// Often quota / billing / key restrictions on free tier.
		if looksLikeQuota(string(respBytes)) {
			return nil, fmt.Errorf("%w: %s", ErrQuotaExceeded, shortErr(respBytes))
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gemini http %d: %s", resp.StatusCode, shortErr(respBytes))
	}

	var apiResp generateContentResponse
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("decode gemini response: %w", err)
	}
	text := apiResp.Text()
	if text == "" {
		return nil, fmt.Errorf("gemini returned empty content")
	}

	ext, err := ParseExtraction(text)
	if err != nil {
		return nil, err
	}
	normalizeExtraction(ext)
	return ext, nil
}

// ParseExtraction unmarshals model JSON (also used in tests).
func ParseExtraction(text string) (*Extraction, error) {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var ext Extraction
	if err := json.Unmarshal([]byte(text), &ext); err != nil {
		return nil, fmt.Errorf("parse extraction json: %w\nraw: %s", err, truncateForErr(text, 400))
	}
	return &ext, nil
}

// Truncate limits body size by runes (not bytes).
func Truncate(s string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return string(runes[:limit]) + "…"
}

func normalizeExtraction(ext *Extraction) {
	ext.Company = strings.TrimSpace(ext.Company)
	ext.Position = strings.TrimSpace(ext.Position)
	ext.Summary = strings.TrimSpace(ext.Summary)
	ext.IgnoreReason = strings.TrimSpace(ext.IgnoreReason)
	ext.Status = strings.ToLower(strings.TrimSpace(ext.Status))
	switch ext.Status {
	case models.StatusApplied, models.StatusOA, models.StatusInterview,
		models.StatusRejected, models.StatusOffer, models.StatusOther:
		// ok
	default:
		if ext.IsJobRelated {
			ext.Status = models.StatusOther
		}
	}
}

func looksLikeQuota(s string) bool {
	l := strings.ToLower(s)
	return strings.Contains(l, "quota") ||
		strings.Contains(l, "rate limit") ||
		strings.Contains(l, "resource_exhausted") ||
		strings.Contains(l, "exceeded")
}

func shortErr(b []byte) string {
	return truncateForErr(string(b), 300)
}

func truncateForErr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

const systemInstruction = `You extract ONLY real job-application status updates from emails.
Return ONLY valid JSON matching the schema. No markdown.

Set is_job_related=true ONLY when the email is clearly one of:
- application received / thanks for applying
- online assessment (OA) / coding challenge invite
- interview invite or interview scheduling
- rejection / not moving forward
- offer

Set is_job_related=false for everything else, including:
- job alerts, digests, "jobs you might like"
- newsletters, marketing, recruiter blasts without a specific application update
- profile/account setup reminders
- generic "stay in consideration" marketing that is not a real interview/OA/status update for a specific role
- LinkedIn/Indeed/Glassdoor notifications that are not status changes

Allowed status values when is_job_related=true: applied, oa, interview, rejected, offer, other.
Keep summary to one short sentence. Prefer null for unknown dates.
Use ISO dates when possible (YYYY-MM-DD or RFC3339).`

func buildPrompt(subject, from string, date time.Time, body string) string {
	dateStr := ""
	if !date.IsZero() {
		dateStr = date.UTC().Format(time.RFC3339)
	}
	return fmt.Sprintf(`Decide if this email is a real application status update, then extract fields.

JSON schema:
{
  "is_job_related": true,
  "company": "Acme Corp",
  "position": "Software Engineer",
  "status": "interview",
  "applied_at": "2026-08-01",
  "interview_at": "2026-08-20T15:00:00Z",
  "oa_at": null,
  "confidence": 0.86,
  "summary": "Invite to interview next week",
  "ignore_reason": null
}

Status mapping (only if is_job_related=true):
- thanks for applying / application received -> applied
- OA / HackerRank / CodeSignal / coding challenge -> oa
- interview invite / schedule -> interview
- reject / unfortunately / not selected -> rejected
- offer -> offer

If it is NOT a clear status update for a specific application, return:
{"is_job_related":false,"company":"","position":"","status":"other","applied_at":null,"interview_at":null,"oa_at":null,"confidence":0.9,"summary":"","ignore_reason":"not a status update"}

Subject: %s
From: %s
Date: %s
Body:
%s
`, subject, from, dateStr, body)
}

type generateContentRequest struct {
	Contents          []content         `json:"contents"`
	GenerationConfig  generationConfig  `json:"generationConfig"`
	SystemInstruction *content          `json:"systemInstruction,omitempty"`
}

type generationConfig struct {
	Temperature      float64 `json:"temperature"`
	MaxOutputTokens  int     `json:"maxOutputTokens"`
	ResponseMIMEType string  `json:"responseMimeType"`
}

type content struct {
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text"`
}

type generateContentResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func (r generateContentResponse) Text() string {
	if len(r.Candidates) == 0 || len(r.Candidates[0].Content.Parts) == 0 {
		return ""
	}
	var b strings.Builder
	for _, p := range r.Candidates[0].Content.Parts {
		b.WriteString(p.Text)
	}
	return b.String()
}
