package auth

import (
	"strings"
	"testing"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("hash = %s", hash)
	}
	if ok, err := VerifyPassword("correct horse", hash); !ok || err != nil {
		t.Fatalf("verify = %v, %v", ok, err)
	}
	if ok, _ := VerifyPassword("wrong horse", hash); ok {
		t.Fatal("wrong password accepted")
	}
	other, _ := HashPassword("correct horse")
	if other == hash {
		t.Fatal("hashes must be salted")
	}
	if _, err := VerifyPassword("x", "$bcrypt$nope"); err == nil {
		t.Fatal("malformed hash accepted")
	}
}

func TestNormalizeEmail(t *testing.T) {
	good := map[string]string{
		" Bob@Example.COM ":       "bob@example.com",
		"a.b+tag@sub.example.org": "a.b+tag@sub.example.org",
	}
	for in, want := range good {
		if got, err := NormalizeEmail(in); err != nil || got != want {
			t.Errorf("NormalizeEmail(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "bob", "bob@localhost", "Bob <bob@example.com>", "a b@example.com", "@example.com"} {
		if _, err := NormalizeEmail(in); err == nil {
			t.Errorf("NormalizeEmail(%q) accepted", in)
		}
	}
}

func TestValidateDisplayName(t *testing.T) {
	for _, ok := range []string{"Bob", "bob_the_builder", "X_1", "abcdefghijklmnop"} {
		if err := ValidateDisplayName(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"ab", "abcdefghijklmnopq", "has space", "émile", "a-b", "ADMIN", "Server"} {
		if err := ValidateDisplayName(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if ValidatePassword("1234567") == nil || ValidatePassword(strings.Repeat("a", 129)) == nil {
		t.Fatal("bad lengths accepted")
	}
	if err := ValidatePassword("12345678"); err != nil {
		t.Fatal(err)
	}
}

func TestChallenge(t *testing.T) {
	c := CodeChallenge("verifier")
	if !ValidChallenge(c) || !ChallengeMatches("verifier", c) || ChallengeMatches("other", c) {
		t.Fatalf("challenge %s", c)
	}
	if ValidChallenge(strings.ToUpper(c)) || ValidChallenge("abc") {
		t.Fatal("invalid challenge accepted")
	}
	if len(NewToken()) != 43 || NewToken() == NewToken() {
		t.Fatal("tokens must be 43 random chars")
	}
}
