package server

import "testing"

func TestSealOpenRoundTrip(t *testing.T) {
	secret := "test-sync-secret-value"
	plain := []byte(`{"email":"a@b.com","token_json":"{}","exp":123}`)
	enc, err := seal(secret, plain)
	if err != nil {
		t.Fatal(err)
	}
	out, err := open(secret, enc)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(plain) {
		t.Fatalf("got %s", out)
	}
	if _, err := open("other-secret", enc); err == nil {
		t.Fatal("expected auth failure")
	}
}
