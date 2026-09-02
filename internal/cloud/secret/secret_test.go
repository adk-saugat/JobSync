package secret

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := "test-sync-secret-value"
	plain := `{"access_token":"abc","refresh_token":"xyz"}`
	enc, err := Encrypt(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	if !IsEncrypted(enc) {
		t.Fatalf("expected prefix, got %q", enc)
	}
	out, err := Decrypt(key, enc)
	if err != nil {
		t.Fatal(err)
	}
	if out != plain {
		t.Fatalf("got %q", out)
	}
	if _, err := Decrypt("other-key", enc); err == nil {
		t.Fatal("expected decrypt failure with wrong key")
	}
}

func TestDecryptPlaintextPassthrough(t *testing.T) {
	plain := "AIzaSyPlaintextKey"
	out, err := Decrypt("any-key", plain)
	if err != nil {
		t.Fatal(err)
	}
	if out != plain {
		t.Fatalf("got %q", out)
	}
}

func TestEncryptEmpty(t *testing.T) {
	enc, err := Encrypt("key", "  ")
	if err != nil || enc != "" {
		t.Fatalf("enc=%q err=%v", enc, err)
	}
}
