package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

// Recover converts a panic in a downstream handler into a 500 response,
// replicating the unhandled-exception half of the .NET ErrorResponseMiddleware
// (GH#418, parity behaviour 1). It runs INSIDE ErrorBody: on a panic with the
// response not yet started, it records the panic message as the request's error
// Detail and sets status 500, leaving ErrorBody to write the
// {"Status":500,"Title":"Internal Server Error","Detail":"<message>"} envelope
// through the same backfill path as the bodyless 4xx responses.
//
// If the response has already started (the handler wrote bytes before
// panicking), the status and body are already on the wire and cannot be
// replaced — matching .NET's `!Response.HasStarted` guard. The panic is logged
// and swallowed so the connection closes cleanly instead of crashing the
// process.
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
				if rw.started {
					// Response already on the wire; cannot replace it. .NET's
					// HasStarted guard skips the envelope in exactly this case.
					return
				}
				SetDetail(ctx, err.Error())
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

// panicError normalises a recovered value into an error whose message mirrors
// what .NET's ex.Message would carry: an error's Error() text, or the value's
// default formatting otherwise.
func panicError(rec any) error {
	if err, ok := rec.(error); ok {
		return err
	}
	return errors.New(fmt.Sprint(rec))
}
