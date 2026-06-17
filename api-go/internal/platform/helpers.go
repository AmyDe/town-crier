package platform

import (
	"context"
	"time"
)

// Sleep waits for d or until ctx is cancelled, whichever comes first. It returns
// the context error when cancelled and nil once the full duration has elapsed.
func Sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Ptr returns a pointer to v. Handy for populating optional struct fields whose
// type is *T from a literal value.
func Ptr[T any](v T) *T { return &v }

// DateOnlyPtrToTime converts an optional DateOnly to the *time.Time the domain
// carries, preserving nil so an absent value stays absent.
func DateOnlyPtrToTime(d *DateOnly) *time.Time {
	if d == nil {
		return nil
	}
	return d.TimePtr()
}
