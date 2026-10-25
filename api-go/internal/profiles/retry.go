package profiles

import (
	"context"
	"errors"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// DefaultSaveAttempts is the number of load-modify-save attempts a caller makes
// before it gives up on a conflicting concurrent write.
const DefaultSaveAttempts = 3

// RetryOnConflict runs fn up to attempts times while it returns
// platform.ErrCASPreconditionFailed. fn must reload the profile on every call.
// Any other error, or the last conflict, is returned as is. It stops early when
// ctx is done.
func RetryOnConflict(ctx context.Context, attempts int, fn func(ctx context.Context) error) error {
	var err error
	for range attempts {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		err = fn(ctx)
		if !errors.Is(err, platform.ErrCASPreconditionFailed) {
			return err
		}
	}
	return err
}
