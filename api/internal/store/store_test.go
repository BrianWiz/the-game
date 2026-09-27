package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestReopenKeepsDataAndSkipsMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api.db")
	ctx := context.Background()
	now := time.Unix(1_800_000_000, 0)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.CreateEmailAccount(ctx, "a@example.com", "hash", "Alice", now)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.AccountByEmail(ctx, "a@example.com")
	if err != nil || got.ID != a.ID || got.DisplayName != "Alice" {
		t.Fatalf("reopened account = %+v, %v", got, err)
	}
}

func TestUniqueness(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	now := time.Unix(1_800_000_000, 0)
	a, _ := s.CreateEmailAccount(ctx, "a@example.com", "h", "Name", now)
	if _, err := s.CreateEmailAccount(ctx, "a@example.com", "h", "Other", now); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("duplicate email: %v", err)
	}
	b, err := s.CreateEmailAccount(ctx, "b@example.com", "h", "NAME", now)
	if err != nil || b.DisplayName != "" {
		t.Fatalf("taken name should leave the new account unnamed: %+v, %v", b, err)
	}
	if err := s.SetDisplayName(ctx, b.ID, "name", now); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("case-insensitive name: %v", err)
	}
	// Several unnamed accounts are fine (NULLs don't collide).
	if _, err := s.CreateDiscordAccount(ctx, "d1", "u", now); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkDiscord(ctx, a.ID, "d1", "u", now); !errors.Is(err, ErrDiscordLinked) {
		t.Fatalf("duplicate discord link: %v", err)
	}
}

func TestTakeIsSingleUseAndPurge(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	now := time.Unix(1_800_000_000, 0)
	a, _ := s.CreateDiscordAccount(ctx, "d1", "u", now)

	s.CreateResetToken(ctx, []byte("h1"), a.ID, now.Add(time.Hour))
	if id, err := s.TakeResetToken(ctx, []byte("h1"), now); err != nil || id != a.ID {
		t.Fatalf("take = %d, %v", id, err)
	}
	if _, err := s.TakeResetToken(ctx, []byte("h1"), now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second take: %v", err)
	}
	s.CreateResetToken(ctx, []byte("h2"), a.ID, now)
	if _, err := s.TakeResetToken(ctx, []byte("h2"), now); !errors.Is(err, ErrExpired) {
		t.Fatalf("stale take: %v", err)
	}

	s.CreateSession(ctx, []byte("s1"), a.ID, now, now.Add(time.Minute))
	if _, err := s.SessionAccount(ctx, []byte("s1"), now); err != nil {
		t.Fatal(err)
	}
	if err := s.PurgeExpired(ctx, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SessionAccount(ctx, []byte("s1"), now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("purged session: %v", err)
	}
}
