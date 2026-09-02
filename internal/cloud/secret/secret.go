package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

const prefixV1 = "enc:v1:"

// KeyFromEnv returns DATA_ENCRYPTION_KEY, or SYNC_SECRET as fallback.
func KeyFromEnv() string {
	if v := strings.TrimSpace(os.Getenv("DATA_ENCRYPTION_KEY")); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("SYNC_SECRET"))
}

// Encrypt seals plaintext with AES-GCM. Empty plaintext stays empty.
// Ciphertext is prefixed so readers can detect encrypted values.
func Encrypt(key, plaintext string) (string, error) {
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		return "", nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("encryption key is empty")
	}
	raw, err := seal([]byte(key), []byte(plaintext))
	if err != nil {
		return "", err
	}
	return prefixV1 + raw, nil
}

// Decrypt opens an Encrypt() value. Plaintext (no prefix) is returned as-is
// so existing Neon rows keep working until the next save re-encrypts them.
func Decrypt(key, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, prefixV1) {
		return value, nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("encryption key is empty")
	}
	raw, err := open([]byte(key), strings.TrimPrefix(value, prefixV1))
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(raw), nil
}

// IsEncrypted reports whether value looks like Encrypt() output.
func IsEncrypted(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), prefixV1)
}

func seal(keyMaterial, plaintext []byte) (string, error) {
	sum := sha256.Sum256(keyMaterial)
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.RawURLEncoding.EncodeToString(out), nil
}

func open(keyMaterial []byte, encoded string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(keyMaterial)
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
