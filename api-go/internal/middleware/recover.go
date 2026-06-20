package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// panicDetail is the fixed generic message written into the client-facing 500
// Detail field on a recovered panic. The actual panic error is always logged
// server-side (see logger.ErrorContext call in Recover); it is deliberately
// never echoed to the client to prevent leaking Go runtime internals (GH#516).
const panicDetail = "An unexpected error occurred."

// Recover converts a panic in a downstream handler into a 500 response
// (GH#418). It runs INSIDE ErrorBody: on a panic with the response not yet
// started, it records a generic Detail and sets status 500, leaving ErrorBody
// to write the {"Status":500,"Title":"Internal Server Error","Detail":"<generic>"}
// envelope through the same backfill path as the bodyless 4xx responses. The
// full panic error is retained in the structured log only (GH#516).
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
					"method", r.Method, "path", r.URL.Path, "error", err)
				// Record the panic on the active request span (started by
				// otelhttp) so it surfaces in App Insights AppExceptions and the
				// request shows as failed (tc-8x8g). When telemetry is disabled
				// SpanFromContext returns a no-op span and these are no-ops.
				span := trace.SpanFromContext(ctx)
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				if rw.started {
					// Response already on the wire; cannot replace it.
					return
				}
				SetDetail(ctx, panicDetail)
				rw.WriteHeader(http.StatusInternalServerError)
			}()
			next.ServeHTTP(rw, r)
		})
	}
}

// recoverWriter tracks whether the response has started so the deferred recover
// knows whether it may still write the 500 envelope. WriteHeader for a >= 400
// status does not mark the response started, because ErrorBody (the next writer
// down) defers flushing those until a body write — so the recover path can
// still set status 500 after a handler called WriteHeader(>=400) and panicked.
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
