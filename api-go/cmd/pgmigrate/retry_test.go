package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

var errLock = &pgconn.PgError{Code: "55P03"}

func TestRetryOnLock_RetriesLockErrorsWithBackoff(t *testing.T) {
	t.Parallel()
	var slept []time.Duration
	calls := 0
	err := retryOnLock(context.Background(), func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}, func(context.Context) error {
		calls++
		if calls < 4 {
			return errLock
		}
		return nil
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second}
	if len(slept) != len(want) {
		t.Fatalf("slept = %v", slept)
	}
	for i := range want {
		if slept[i] != want[i] {
			t.Fatalf("slept = %v, want %v", slept, want)
		}
	}
}

func TestRetryOnLock_GivesUpAfterFiveAttempts(t *testing.T) {
	t.Parallel()
	calls := 0
	var slept []time.Duration
	err := retryOnLock(context.Background(), func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}, func(context.Context) error {
		calls++
		return errLock
	})
	if !errors.Is(err, errLock) || calls != 5 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	if len(slept) != 4 || slept[3] != 40*time.Second {
		t.Fatalf("slept = %v", slept)
	}
}

func TestRetryOnLock_DoesNotRetryOtherErrors(t *testing.T) {
	t.Parallel()
	calls := 0
	boom := errors.New("boom")
	err := retryOnLock(context.Background(), func(context.Context, time.Duration) error { return nil }, func(context.Context) error {
		calls++
		return boom
	})
	if !errors.Is(err, boom) || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestRetryOnLock_StopsWhenSleepFails(t *testing.T) {
	t.Parallel()
	calls := 0
	err := retryOnLock(context.Background(), func(context.Context, time.Duration) error { return context.DeadlineExceeded }, func(context.Context) error {
		calls++
		return errLock
	})
	if !errors.Is(err, context.DeadlineExceeded) || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestInvalidIndexError(t *testing.T) {
	t.Parallel()
	if err := invalidIndexError(nil); err != nil {
		t.Fatalf("err = %v", err)
	}
	err := invalidIndexError([]string{"public.a", "public.b"})
	if err == nil || !strings.Contains(err.Error(), "public.a") || !strings.Contains(err.Error(), "public.b") {
		t.Fatalf("err = %v", err)
	}
}
