package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// DefaultQuery is a conservative V1 search for job-related mail.
const DefaultQuery = `(application OR applied OR interview OR "coding challenge" OR assessment OR "online assessment" OR OA OR unfortunately OR "next steps" OR offer OR recruiter) newer_than:14d -category:promotions`

// MessageMeta is lightweight search result data.
type MessageMeta struct {
	ID       string
	ThreadID string
	Date     time.Time
}

// Message is a full email payload used for later Gemini extraction.
type Message struct {
	ID      string
	ThreadID string
	Subject string
	From    string
	Date    time.Time
	Snippet string
	Body    string
}

// Client wraps the Gmail API.
type Client struct {
	svc *gmail.Service
}

// NewClient builds a Gmail client from an authenticated HTTP client.
func NewClient(ctx context.Context, httpClient *http.Client) (*Client, error) {
	svc, err := gmail.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("gmail service: %w", err)
	}
	return &Client{svc: svc}, nil
}

// Search returns recent candidate messages, newest first, up to limit.
// after, when non-zero, keeps only messages with Date strictly after that time.
func (c *Client) Search(ctx context.Context, query string, limit int64, after time.Time) ([]MessageMeta, error) {
	if query == "" {
		query = DefaultQuery
	}
	if limit <= 0 {
		limit = 15
	}

	call := c.svc.Users.Messages.List("me").Q(query).MaxResults(limit).Context(ctx)
	resp, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("gmail search: %w", err)
	}

	out := make([]MessageMeta, 0, len(resp.Messages))
	for _, m := range resp.Messages {
		if m == nil || m.Id == "" {
			continue
		}
		meta, err := c.getMeta(ctx, m.Id)
		if err != nil {
			return nil, err
		}
		if !after.IsZero() && !meta.Date.After(after) {
			continue
		}
		out = append(out, meta)
	}
	return out, nil
}

// GetMessage loads subject/from/date/body for one message.
func (c *Client) GetMessage(ctx context.Context, id string) (*Message, error) {
	raw, err := c.svc.Users.Messages.Get("me", id).Format("full").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("gmail get %s: %w", id, err)
	}

	msg := &Message{
		ID:       raw.Id,
		ThreadID: raw.ThreadId,
		Snippet:  raw.Snippet,
		Date:     time.UnixMilli(raw.InternalDate).UTC(),
		Subject:  header(raw, "Subject"),
		From:     header(raw, "From"),
		Body:     extractBody(raw),
	}
	return msg, nil
}

func (c *Client) getMeta(ctx context.Context, id string) (MessageMeta, error) {
	raw, err := c.svc.Users.Messages.Get("me", id).Format("metadata").
		MetadataHeaders("Date", "Subject", "From").
		Context(ctx).Do()
	if err != nil {
		return MessageMeta{}, fmt.Errorf("gmail meta %s: %w", id, err)
	}
	return MessageMeta{
		ID:       raw.Id,
		ThreadID: raw.ThreadId,
		Date:     time.UnixMilli(raw.InternalDate).UTC(),
	}, nil
}

func header(msg *gmail.Message, name string) string {
	if msg.Payload == nil {
		return ""
	}
	for _, h := range msg.Payload.Headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

func extractBody(msg *gmail.Message) string {
	if msg.Payload == nil {
		return msg.Snippet
	}
	if plain := findPart(msg.Payload, "text/plain"); plain != "" {
		return plain
	}
	if html := findPart(msg.Payload, "text/html"); html != "" {
		return stripTags(html)
	}
	if msg.Payload.Body != nil && msg.Payload.Body.Data != "" {
		if decoded, err := decodeBody(msg.Payload.Body.Data); err == nil && decoded != "" {
			return decoded
		}
	}
	return msg.Snippet
}

func findPart(part *gmail.MessagePart, mime string) string {
	if part == nil {
		return ""
	}
	if strings.EqualFold(part.MimeType, mime) && part.Body != nil && part.Body.Data != "" {
		if decoded, err := decodeBody(part.Body.Data); err == nil {
			return decoded
		}
	}
	for _, child := range part.Parts {
		if got := findPart(child, mime); got != "" {
			return got
		}
	}
	return ""
}

func decodeBody(data string) (string, error) {
	// Gmail uses URL-safe base64.
	raw, err := base64.URLEncoding.DecodeString(data)
	if err != nil {
		raw, err = base64.RawURLEncoding.DecodeString(data)
		if err != nil {
			return "", err
		}
	}
	return string(raw), nil
}

func stripTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// LooksJobRelated is a cheap local pre-filter before Gemini (Phase 4+).
func LooksJobRelated(subject, from string) bool {
	s := strings.ToLower(subject + " " + from)
	keywords := []string{
		"application", "applied", "interview", "assessment", "coding challenge",
		"hackerrank", "codesignal", "oa ", " online assessment", "unfortunately",
		"next steps", "offer", "recruiter", "candidacy", "position", "role",
		"thank you for applying", "thanks for applying", "move forward",
	}
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}
