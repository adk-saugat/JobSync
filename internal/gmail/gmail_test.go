package gmail

import "testing"

func TestLooksJobRelated(t *testing.T) {
	cases := []struct {
		subject, from string
		want          bool
	}{
		{"Thanks for applying to Acme", "jobs@acme.com", true},
		{"Interview invitation", "recruiter@x.com", true},
		{"Your receipt from Amazon", "auto@amazon.com", false},
		{"Weekly newsletter", "news@foo.com", false},
	}
	for _, tc := range cases {
		if got := LooksJobRelated(tc.subject, tc.from); got != tc.want {
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
