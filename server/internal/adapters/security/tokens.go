package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// Tokens makes session tokens: 32 random bytes in base64url; only their SHA-256 is stored.
type Tokens struct{}

func NewTokens() Tokens { return Tokens{} }

func (Tokens) New() (string, []byte) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	plain := base64.RawURLEncoding.EncodeToString(b)
	return plain, Tokens{}.Hash(plain)
}

func (Tokens) Hash(plain string) []byte {
	sum := sha256.Sum256([]byte(plain))
	return sum[:]
}
