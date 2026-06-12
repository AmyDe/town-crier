package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
)

// activityRecorder updates a user's last-active timestamp. The concrete
// implementation (built over the profile store) dedupes writes within a 24h
// window and is the .NET RecordUserActivityCommandHandler equivalent; the
// middleware only reports the (user, time) and never inspects the result.
type activityRecorder interface {
	RecordActivity(ctx context.Context, userID string, at time.Time) error
}

// RecordActivity returns middleware that, after the inner handler completes,
// records the authenticated user's activity for the dormancy-cleanup worker (UK
// GDPR Art. 5(1)(e)). It mirrors .NET's RecordUserActivityMiddleware: runs after
// the response, skips anonymous requests, and swallows recorder failures so a
// transient Cosmos error never turns a successful request into a 500.
func RecordActivity(recorder activityRecorder, now func() time.Time, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)

			userID := auth.Subject(r.Context())
			if userID == "" {
				return
			}

			// Use a context detached from the request so cancellation after the
			// response is sent does not abort the activity write; bound it instead.
			ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), activityWriteTimeout)
			defer cancel()
			if err := recorder.RecordActivity(ctx, userID, now()); err != nil {
				logger.WarnContext(ctx, "failed to record user activity", "userId", userID, "error", err)
			}
		})
	}
}

// activityWriteTimeout bounds the post-response activity write.
const activityWriteTimeout = 5 * time.Second
