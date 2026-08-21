package domain

import "testing"

func TestShouldUpdateStatus(t *testing.T) {
	cases := []struct {
		current, next string
		want          bool
	}{
		{StatusApplied, StatusInterview, true},
		{StatusInterview, StatusApplied, false},
		{StatusApplied, StatusRejected, true},
		{StatusAccepted, StatusRejected, false},
		{StatusInterview, StatusAccepted, true},
		{StatusApplied, StatusAssessment, true},
		{StatusAssessment, StatusInterview, true},
		{"", StatusApplied, true},
		{StatusApplied, StatusApplied, false},
	}
	for _, tc := range cases {
		if got := ShouldUpdateStatus(tc.current, tc.next); got != tc.want {
			t.Fatalf("%s -> %s: got %v want %v", tc.current, tc.next, got, tc.want)
		}
	}
}
