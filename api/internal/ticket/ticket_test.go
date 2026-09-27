package ticket

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Shared test vector: game/tests/unit/test_join_ticket.gd verifies this exact ticket with
// this key, so the Go signer and the GDScript verifier can't drift apart silently.
const (
	vectorKey    = "0123456789abcdef0123456789abcdef"
	vectorTicket = "v1.eyJhaWQiOjQyLCJuYW1lIjoiQm9iX3RoZV9CdWlsZGVyIiwiZXhwIjoyMDAwMDAwMDAwLCJub25jZSI6Im5vbmNlLTEyMyJ9.A7eknlwKGIu3Q+/X3P4pFog5M/yLwv/cQK4SQuKBzNE="
)

var vectorClaims = Claims{AccountID: 42, Name: "Bob_the_Builder", Expires: 2000000000, Nonce: "nonce-123"}

func TestSignMatchesSharedVector(t *testing.T) {
	got, err := Sign([]byte(vectorKey), vectorClaims)
	if err != nil {
		t.Fatal(err)
	}
	if got != vectorTicket {
		t.Fatalf("ticket changed; update the GDScript test vector too\n got %s\nwant %s", got, vectorTicket)
	}
}

func TestVerifyRoundTrip(t *testing.T) {
	now := time.Unix(1999999000, 0)
	c, err := Verify([]byte(vectorKey), vectorTicket, now)
	if err != nil {
		t.Fatal(err)
	}
	if c != vectorClaims {
		t.Fatalf("claims = %+v", c)
	}
}

func TestVerifyRejects(t *testing.T) {
	now := time.Unix(1999999000, 0)
	cases := map[string]struct {
		key, ticket string
		now         time.Time
	}{
		"wrong key":  {"fedcba9876543210fedcba9876543210", vectorTicket, now},
		"tampered":   {vectorKey, vectorTicket[:10] + "x" + vectorTicket[11:], now},
		"expired":    {vectorKey, vectorTicket, time.Unix(2000000000, 0)},
		"no version": {vectorKey, vectorTicket[3:], now},
		"garbage":    {vectorKey, "v1.", now},
	}
	for name, tc := range cases {
		if _, err := Verify([]byte(tc.key), tc.ticket, tc.now); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func TestLoadKey(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good")
	os.WriteFile(good, []byte(vectorKey+"\n"), 0o600)
	key, err := LoadKey(good)
	if err != nil || string(key) != vectorKey {
		t.Fatalf("LoadKey = %q, %v", key, err)
	}
	short := filepath.Join(dir, "short")
	os.WriteFile(short, []byte("too short\n"), 0o600)
	if _, err := LoadKey(short); err == nil {
		t.Fatal("short key accepted")
	}
}
