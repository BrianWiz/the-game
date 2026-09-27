// Package ticket signs game join tickets. The Godot server verifies them with the same
// shared key (game/core/net/join_ticket.gd), so any format change must land in both.
//
// Format: "v1." + base64(payload) + "." + base64(HMAC-SHA256(key, "v1." + base64(payload)))
// using standard, padded base64, where payload is JSON:
//
//	{"aid":<account id>,"name":"<display name>","exp":<unix seconds>,"nonce":"<random>"}
//
// HMAC rather than Ed25519 because Godot's Crypto class can't verify Ed25519, and the API
// and game server run on the same host.
package ticket

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const prefix = "v1."

// MinKeyLen is the shortest accepted key (bytes of the trimmed key file).
const MinKeyLen = 32

var ErrInvalid = errors.New("invalid ticket")

type Claims struct {
	AccountID int64  `json:"aid"`
	Name      string `json:"name"`
	Expires   int64  `json:"exp"`
	Nonce     string `json:"nonce"`
}

// Sign returns the ticket string for claims.
func Sign(key []byte, c Claims) (string, error) {
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	msg := prefix + base64.StdEncoding.EncodeToString(payload)
	return msg + "." + base64.StdEncoding.EncodeToString(mac(key, msg)), nil
}

// Verify checks the signature and expiry. The game server does the same (plus nonce
// replay checks); this exists for tests and tooling.
func Verify(key []byte, t string, now time.Time) (Claims, error) {
	dot := strings.LastIndexByte(t, '.')
	if !strings.HasPrefix(t, prefix) || dot <= len(prefix) {
		return Claims{}, ErrInvalid
	}
	msg := t[:dot]
	sig, err := base64.StdEncoding.DecodeString(t[dot+1:])
	if err != nil || !hmac.Equal(sig, mac(key, msg)) {
		return Claims{}, ErrInvalid
	}
	payload, err := base64.StdEncoding.DecodeString(msg[len(prefix):])
	if err != nil {
		return Claims{}, ErrInvalid
	}
	var c Claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return Claims{}, ErrInvalid
	}
	if c.Expires <= now.Unix() {
		return Claims{}, fmt.Errorf("%w: expired", ErrInvalid)
	}
	return c, nil
}

func mac(key []byte, msg string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(msg))
	return h.Sum(nil)
}

// LoadKey reads a key file. The key is the file's trimmed contents as bytes (for example
// the output of `openssl rand -hex 32`), exactly as the game server reads it.
func LoadKey(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	key := bytes.TrimSpace(raw)
	if len(key) < MinKeyLen {
		return nil, fmt.Errorf("ticket key in %s is shorter than %d bytes", path, MinKeyLen)
	}
	return key, nil
}
