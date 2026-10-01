package token

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

const Bytes = 32

// New returns a fresh 256-bit URL-safe base64 token (raw, no padding).
func New() (string, error) {
	b := make([]byte, Bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Hash returns hex(HMAC-SHA256(pepper, token)). Used as the Firestore document
// ID for the token row so a Firestore leak yields hashes, not usable URLs.
func Hash(pepper []byte, token string) string {
	h := hmac.New(sha256.New, pepper)
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}
