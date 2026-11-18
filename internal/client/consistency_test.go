package client

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestEventually_SucceedsAfterTransientMisses(t *testing.T) {
	calls := 0
	err := eventually(context.Background(), 6, time.Millisecond, time.Millisecond, func() (bool, error) {
		calls++
		return calls >= 3, nil // not found on the first two polls, then appears
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestEventually_GivesUpAfterMaxAttempts(t *testing.T) {
	calls := 0
	err := eventually(context.Background(), 4, time.Millisecond, time.Millisecond, func() (bool, error) {
		calls++
		return false, nil
	})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if calls != 4 {
		t.Fatalf("expected 4 calls (one per attempt), got %d", calls)
	}
}

func TestEventually_PropagatesError(t *testing.T) {
	sentinel := errors.New("boom")
	calls := 0
	err := eventually(context.Background(), 6, time.Millisecond, time.Millisecond, func() (bool, error) {
		calls++
		return false, sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected to stop after first error, got %d calls", calls)
	}
}

func TestEventually_AbortsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	calls := 0
	err := eventually(ctx, 6, 10*time.Millisecond, 10*time.Millisecond, func() (bool, error) {
		calls++
		return false, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	// fn runs once before the first backoff/select observes cancellation.
	if calls != 1 {
		t.Fatalf("expected 1 call before cancel, got %d", calls)
	}
}
