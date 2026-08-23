package auth

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// GoogleEmailFromTokenJSON returns the Google account email for a saved OAuth token.
// Uses Gmail profile (same scopes as init/sync) — not userinfo, which needs extra scopes.
func GoogleEmailFromTokenJSON(ctx context.Context, tokenJSON []byte) (string, error) {
	client, err := HTTPClientFromStoredToken(ctx, string(tokenJSON), nil)
	if err != nil {
		return "", err
	}
	svc, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return "", fmt.Errorf("gmail service: %w", err)
	}
	profile, err := svc.Users.GetProfile("me").Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("gmail profile: %w", err)
	}
	email := strings.TrimSpace(profile.EmailAddress)
	if email == "" {
		return "", fmt.Errorf("gmail profile: email missing")
	}
	return email, nil
}

var accountIDSanitizer = regexp.MustCompile(`[^a-z0-9._+-]+`)

// AccountIDFromEmail converts an email into a stable Neon account id.
func AccountIDFromEmail(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	email = strings.ReplaceAll(email, "@", "_at_")
	return accountIDSanitizer.ReplaceAllString(email, "_")
}
