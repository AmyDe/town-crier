package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// panicDetail is the client-facing 500 Detail on a recovered panic; the real
// error is only logged, so Go runtime internals never leak to the client.
const panicDetail = "An unexpected error occurred."

// Recover converts a panic in a downstream handler into a 500 response.
// It runs INSIDE ErrorBody: on a panic with the response not yet
// started, it records a generic Detail and sets status 500, leaving ErrorBody
// to write the {"Status":500,"Title":"Internal Server Error","Detail":"<generic>"}
// envelope through the same backfill path as the bodyless 4xx responses. The
// full panic error is retained in the structured log only.
//
// If the response has already started (the handler wrote bytes before
// panicking), the status and body are already on the wire and cannot be
// replaced. The panic is logged and swallowed so the connection closes cleanly
// instead of crashing the process.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			rw := &recoverWriter{ResponseWriter: w}
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				err := panicError(rec)
				logger.ErrorContext(ctx, "recovered from panic",
					"method", r.Method, "path", RedactPath(r.URL.Path), "error", err)
				span := trace.SpanFromContext(ctx)
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				if rw.started {
					return
				}
				SetDetail(ctx, panicDetail)
				rw.WriteHeader(http.StatusInternalServerError)
			}()
			next.ServeHTTP(rw, r)
		})
	}
}

// recoverWriter tracks whether the response has started. A >= 400 WriteHeader
// does not count, because ErrorBody defers flushing it, so a later panic can
// still become a 500.
type recoverWriter struct {
	http.ResponseWriter
	started bool
}

func (w *recoverWriter) WriteHeader(status int) {
	if status < 400 {
		w.started = true
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *recoverWriter) Write(p []byte) (int, error) {
	w.started = true
	return w.ResponseWriter.Write(p)
}

// Unwrap exposes the wrapped writer to http.ResponseController and to ErrorBody's
// own unwrapping, so flush and the deferred-status machinery keep working.
func (w *recoverWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// panicError normalises a recovered value into an error: an error's Error()
// text, or the value's default formatting otherwise.
func panicError(rec any) error {
	if err, ok := rec.(error); ok {
		return err
	}
	return errors.New(fmt.Sprint(rec))
}
