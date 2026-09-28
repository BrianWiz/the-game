package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestReserveLimits(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	now := time.Unix(1_800_000_000, 0)
	lim := Limits{PerUser: 2, Window: 24 * time.Hour, MaxActive: 2, StaleAfter: 3 * time.Hour}

	r1, err := s.Reserve(ctx, "a", 0, "implement", lim, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reserve(ctx, "b", 0, "implement", lim, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var le *LimitError
	if _, err := s.Reserve(ctx, "c", 0, "implement", lim, now); !errors.As(err, &le) || !le.Active {
		t.Fatalf("want active limit, got %v", err)
	}
	// A finished run frees its active slot but still counts for its user.
	if err := s.SetRunStatus(ctx, r1.ID, RunCompleted, "success", 9, "u", now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reserve(ctx, "a", 0, "implement", lim, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, err = s.Reserve(ctx, "a", 0, "implement", lim, now.Add(3*time.Minute))
	if !errors.As(err, &le) || le.Active || !le.Retry.Equal(now.Add(24*time.Hour)) {
		t.Fatalf("want per-user limit retrying at +24h, got %v %+v", err, le)
	}
	// Runs stuck active past StaleAfter stop counting toward the cap.
	later := now.Add(4 * time.Hour)
	if _, err := s.Reserve(ctx, "c", 0, "implement", lim, later); err != nil {
		t.Fatalf("stale runs still counted: %v", err)
	}
	// And the window rolls.
	if _, err := s.Reserve(ctx, "a", 0, "implement", Limits{PerUser: 2, Window: 24 * time.Hour, MaxActive: 5, StaleAfter: time.Hour}, now.Add(25*time.Hour)); err != nil {
		t.Fatalf("window did not roll: %v", err)
	}
}

func TestJobsAndRuns(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	now := time.Unix(1_800_000_000, 0)
	r, err := s.Reserve(ctx, "u", 0, "implement", Limits{Window: time.Hour}, now)
	if err != nil {
		t.Fatal(err)
	}
	j, err := s.CreateJob(ctx, Job{Issue: 3, Title: "t", ChannelID: "c", RequesterID: "u", RequesterName: "U"}, r.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetThread(ctx, j.ID, "th", now); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPR(ctx, j.ID, 4, now); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{3, 4} {
		got, err := s.JobByNumber(ctx, n)
		if err != nil || got.ID != j.ID || got.ThreadID != "th" || got.PR != 4 || got.State != JobOpen {
			t.Errorf("JobByNumber(%d) = %+v %v", n, got, err)
		}
	}
	active, err := s.ActiveRunForJob(ctx, j.ID)
	if err != nil || active.ID != r.ID {
		t.Fatalf("active run %+v %v", active, err)
	}
	s.SetRunStatus(ctx, r.ID, RunInProgress, "", 11, "url", now)
	s.SetRunStatus(ctx, r.ID, RunCompleted, "success", 0, "", now)
	got, _ := s.RunByID(ctx, r.ID)
	if got.WorkflowRunID != 11 || got.RunURL != "url" || got.Conclusion != "success" {
		t.Errorf("run details lost: %+v", got)
	}
	if _, err := s.ActiveRunForJob(ctx, j.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("finished run still active: %v", err)
	}
}

func TestSeen(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	now := time.Unix(1_800_000_000, 0)
	if fresh, _ := s.MarkSeen(ctx, "k", now); !fresh {
		t.Error("first mark not fresh")
	}
	if fresh, _ := s.MarkSeen(ctx, "k", now); fresh {
		t.Error("second mark fresh")
	}
	s.Purge(ctx, now.Add(48*time.Hour), 24*time.Hour)
	if fresh, _ := s.MarkSeen(ctx, "k", now); !fresh {
		t.Error("purged key still seen")
	}
}
