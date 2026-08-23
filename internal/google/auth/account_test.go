package auth

import "testing"

func TestAccountIDFromEmail(t *testing.T) {
	got := AccountIDFromEmail("Jane.Doe+jobs@Example.com")
	want := "jane.doe+jobs_at_example.com"
	if got != want {
		t.Fatalf("AccountIDFromEmail = %q, want %q", got, want)
	}
}
