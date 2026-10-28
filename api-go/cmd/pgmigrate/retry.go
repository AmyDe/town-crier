package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres"
)

// lockRetryBackoffs are the waits between the five attempts at a migration that
// failed with lock_not_available.
var lockRetryBackoffs = []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("wait for retry: %w", ctx.Err())
	case <-t.C:
		return nil
	}
}

// retryOnLock runs fn and retries it only when it fails with SQLSTATE 55P03.
func retryOnLock(ctx context.Context, sleep func(context.Context, time.Duration) error, fn func(context.Context) error) error {
	for attempt := 0; ; attempt++ {
		err := fn(ctx)
		if err == nil || !postgres.IsLockNotAvailable(err) || attempt >= len(lockRetryBackoffs) {
			return err
		}
		if sleepErr := sleep(ctx, lockRetryBackoffs[attempt]); sleepErr != nil {
			return sleepErr
		}
	}
}

func invalidIndexError(names []string) error {
	if len(names) == 0 {
		return nil
	}
	return fmt.Errorf("invalid indexes found: %s", strings.Join(names, ", "))
}
