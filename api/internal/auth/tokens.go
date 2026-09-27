package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
)

// NewToken returns 32 random bytes as unpadded base64url (43 characters).
func NewToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// HashToken is how tokens are stored: SHA-256, so the database never holds usable secrets.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// CodeChallenge derives the challenge a client sends for a verifier: lowercase hex
// SHA-256. (Godot can compute it with String.sha256_text().)
func CodeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return hex.EncodeToString(sum[:])
}

// ChallengeMatches checks a verifier against a stored challenge in constant time.
func ChallengeMatches(verifier, challenge string) bool {
	return subtle.ConstantTimeCompare([]byte(CodeChallenge(verifier)), []byte(challenge)) == 1
}

// ValidChallenge reports whether s looks like a CodeChallenge (64 lowercase hex chars).
func ValidChallenge(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
