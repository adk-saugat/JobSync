package gmail

import "testing"

func TestLooksLikeStatusUpdate(t *testing.T) {
	cases := []struct {
		subject, from string
		want          bool
	}{
		{"Thanks for applying to Acme", "jobs@acme.com", true},
		{"Your IBM Application Status", "talent@ibm.com", true},
		{"Interview invitation — Software Engineer", "recruiter@x.com", true},
		{"Unfortunately we will not move forward", "hr@x.com", true},
		{"HackerRank invitation", "no-reply@hackerrank.com", true},
		{"Complete your interview to stay in consideration", "support@micro1.ai", true},
		{"Saugat Adhikari, You're invited! Assessment for Software Engineer Intern (Recommendation Infra, Performance Efficiency) - 2027 Summer - TikTok Early Careers", "TikTok <job@careers.tiktok.com>", true},
		{"Complete your assessment — Software Engineer", "no-reply@codility.com", true},
		{"Weekly newsletter", "news@foo.com", false},
		{"10 new jobs for you", "alerts@indeed.com", false},
		{"Jobs you may be interested in", "jobs-noreply@linkedin.com", false},
		{"Complete your profile", "noreply@linkedin.com", false},
		{"Your receipt from Amazon", "auto@amazon.com", false},
	}
	for _, tc := range cases {
		if got := LooksLikeStatusUpdate(tc.subject, tc.from); got != tc.want {
			t.Fatalf("%q / %q: got %v want %v", tc.subject, tc.from, got, tc.want)
		}
	}
}

func TestStripTags(t *testing.T) {
	got := stripTags("<p>Hello <b>world</b></p>")
	if got != "Hello world" {
		t.Fatalf("got %q", got)
	}
}
