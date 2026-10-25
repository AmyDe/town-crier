package profiles

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

func TestRetryOnConflict(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	conflict := fmt.Errorf("save: %w", platform.ErrCASPreconditionFailed)

	tests := []struct {
		name      string
		results   []error
		wantCalls int
		wantErr   error
	}{
		{name: "success first time", results: []error{nil}, wantCalls: 1},
		{name: "retries conflict then succeeds", results: []error{conflict, conflict, nil}, wantCalls: 3},
		{name: "gives up after three conflicts", results: []error{conflict, conflict, conflict, nil}, wantCalls: 3, wantErr: platform.ErrCASPreconditionFailed},
		{name: "other error is not retried", results: []error{boom, nil}, wantCalls: 1, wantErr: boom},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			err := RetryOnConflict(context.Background(), 3, func(context.Context) error {
				res := tc.results[calls]
				calls++
				return res
			})
			if calls != tc.wantCalls {
				t.Errorf("calls: got %d, want %d", calls, tc.wantCalls)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("err: got %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestRetryOnConflict_StopsWhenContextCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := RetryOnConflict(ctx, 3, func(context.Context) error {
		calls++
		cancel()
		return platform.ErrCASPreconditionFailed
	})
	if calls != 1 {
		t.Errorf("calls: got %d, want 1", calls)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err: got %v, want context.Canceled", err)
	}
}
