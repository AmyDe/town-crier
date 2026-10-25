package worker

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"
)

type deadlineRecorder struct{ remaining time.Duration }

func (d *deadlineRecorder) record(ctx context.Context) {
	if dl, ok := ctx.Deadline(); ok {
		d.remaining = time.Until(dl)
	}
}

func (d *deadlineRecorder) RunWeekly(ctx context.Context) error { d.record(ctx); return nil }
func (d *deadlineRecorder) RunHourly(ctx context.Context) error { d.record(ctx); return nil }
func (d *deadlineRecorder) Run(ctx context.Context) (int, error) {
	d.record(ctx)
	return 0, nil
}

type deadlinePurge struct{ deadlineRecorder }

func (d *deadlinePurge) Run(ctx context.Context) (int, int, error) {
	d.record(ctx)
	return 0, 0, nil
}

type deadlineReconcile struct{ deadlineRecorder }

func (d *deadlineReconcile) Run(ctx context.Context) (int, int, int, error) {
	d.record(ctx)
	return 0, 0, 0, nil
}

func TestRun_CycleBudgetsStayUnderReplicaTimeouts(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	tests := []struct {
		mode string
		max  time.Duration
		run  func(rec *deadlineRecorder) int
	}{
		{"hourly-digest", 4 * time.Minute, func(r *deadlineRecorder) int {
			return Run(context.Background(), "hourly-digest", r, nil, nil, nil, nil, nil, nil, logger)
		}},
		{"digest", 8 * time.Minute, func(r *deadlineRecorder) int {
			return Run(context.Background(), "digest", r, nil, nil, nil, nil, nil, nil, logger)
		}},
		{"dormant-cleanup", 8 * time.Minute, func(r *deadlineRecorder) int {
			return Run(context.Background(), "dormant-cleanup", nil, r, nil, nil, nil, nil, nil, logger)
		}},
		{"subscription-sweep", 8 * time.Minute, func(r *deadlineRecorder) int {
			return Run(context.Background(), "subscription-sweep", nil, nil, nil, r, nil, nil, nil, logger)
		}},
		{"pg-purge", 8 * time.Minute, func(r *deadlineRecorder) int {
			p := &deadlinePurge{}
			code := Run(context.Background(), "pg-purge", nil, nil, nil, nil, p, nil, nil, logger)
			r.remaining = p.remaining
			return code
		}},
		{"appstore-reconcile", 8 * time.Minute, func(r *deadlineRecorder) int {
			p := &deadlineReconcile{}
			code := Run(context.Background(), "appstore-reconcile", nil, nil, nil, nil, nil, p, nil, logger)
			r.remaining = p.remaining
			return code
		}},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			t.Parallel()
			rec := &deadlineRecorder{}
			if code := tt.run(rec); code != 0 {
				t.Fatalf("exit code: got %d, want 0", code)
			}
			if rec.remaining <= 0 || rec.remaining > tt.max {
				t.Errorf("budget: got %v, want within (0, %v]", rec.remaining, tt.max)
			}
		})
	}
}
