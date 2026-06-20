// Package middleware holds the cross-cutting http.Handler wrappers (GH#418).
// Composition is plain func(http.Handler) http.Handler, chained by hand in
// cmd/api/main.go.
package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// errorResponse is the error envelope: PascalCase keys in declaration order,
// Detail serialized as an explicit null when unset. Detail is populated only
// from the unhandled-exception (panic recovery) path.
type errorResponse struct {
	Status int     `json:"Status"`
	Title  string  `json:"Title"`
	Detail *string `json:"Detail"`
}

// detailHolder is a per-request mailbox for the error Detail. Go contexts are
// immutable, so a pointer installed once by ErrorBody and mutated by Recover
// (which runs further down the chain) is the shared-write channel. ErrorBody
// reads it after next returns.
type detailHolder struct {
	detail *string
}

type detailKey struct{}

// SetDetail records the error detail for the current request so ErrorBody emits
// it as the envelope's Detail field. It is a no-op when the request did not
// pass through ErrorBody (no holder installed), keeping callers decoupled from
// the middleware ordering.
func SetDetail(ctx context.Context, detail string) {
	if h, ok := ctx.Value(detailKey{}).(*detailHolder); ok {
		h.detail = &detail
	}
}

// ErrorBody backfills any response with status >= 400 that would otherwise be
// sent with an empty body (GH#418): it adds the PascalCase JSON envelope
// {"Status":<n>,"Title":"<reason>","Detail":null} with Content-Type exactly
// "application/json" (no charset on this path, unlike handler-written bodies).
// Responses that already carry a body pass through untouched.
func ErrorBody(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			holder := &detailHolder{}
			bw := &backfillWriter{ResponseWriter: w, holder: holder}
			next.ServeHTTP(bw, r.WithContext(context.WithValue(ctx, detailKey{}, holder)))
			if err := bw.backfill(); err != nil {
				logger.ErrorContext(ctx, "write error body", "status", bw.status, "error", err)
			}
			// Flag the request span as failed for any server error (>= 500) so it
			// shows as failed in App Insights AppRequests and is queryable
			// (tc-8x8g task D). This is the single central hook — handlers don't
			// touch the span. 4xx are client errors and are deliberately left
			// unflagged. When telemetry is disabled SpanFromContext returns a
			// no-op span and this is a no-op.
			//
			// The panic path (Recover, which runs inside this middleware) already
			// set Error status with the richer panic message and recorded the
			// exception; a non-nil holder.detail is that signal, so we skip the
			// generic re-set to avoid clobbering the panic message.
			if bw.status >= http.StatusInternalServerError && holder.detail == nil {
				trace.SpanFromContext(ctx).SetStatus(codes.Error, http.StatusText(bw.status))
			}
		})
	}
}

// backfillWriter defers WriteHeader for >= 400 statuses until the handler
// either writes body bytes or returns. net/http flushes headers on WriteHeader,
// so deferring is what lets us check whether the response has started yet (and
// apply a late Content-Type overwrite) before the header is sent.
type backfillWriter struct {
	http.ResponseWriter
	holder    *detailHolder // shared mailbox carrying the panic Detail, if any
	status    int           // deferred status; 0 = WriteHeader not called yet
	flushed   bool          // header forwarded to the underlying writer
	wroteBody bool
}

func (b *backfillWriter) WriteHeader(status int) {
	if b.flushed || b.status != 0 {
		return // first WriteHeader wins, as in net/http
	}
	b.status = status
	if status < 400 {
		b.flushed = true
		b.ResponseWriter.WriteHeader(status)
	}
}

func (b *backfillWriter) Write(p []byte) (int, error) {
	if !b.flushed {
		if b.status != 0 {
			b.ResponseWriter.WriteHeader(b.status)
		}
		b.flushed = true
	}
	if len(p) > 0 {
		b.wroteBody = true
	}
	return b.ResponseWriter.Write(p)
}

// Unwrap lets http.ResponseController reach the underlying writer (Flush etc.).
func (b *backfillWriter) Unwrap() http.ResponseWriter { return b.ResponseWriter }

// backfill writes the error envelope if the handler finished a >= 400 response
// without a body.
func (b *backfillWriter) backfill() error {
	if b.flushed || b.status < 400 || b.wroteBody {
		return nil
	}
	var detail *string
	if b.holder != nil {
		detail = b.holder.detail
	}
	body, err := json.Marshal(errorResponse{Status: b.status, Title: reasonPhrase(b.status), Detail: detail})
	if err != nil {
		b.flushed = true
		b.ResponseWriter.WriteHeader(b.status)
		return fmt.Errorf("marshal error body: %w", err)
	}
	b.Header().Set("Content-Type", "application/json")
	b.flushed = true
	b.ResponseWriter.WriteHeader(b.status)
	if _, err := b.ResponseWriter.Write(body); err != nil {
		return fmt.Errorf("write error body: %w", err)
	}
	return nil
}

// reasonPhrase returns the reason-phrase title for the given status code. 429
// is deliberately absent, so it — like every other unmapped status — falls
// through to "Error".
func reasonPhrase(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "Bad Request"
	case http.StatusUnauthorized:
		return "Unauthorized"
	case http.StatusForbidden:
		return "Forbidden"
	case http.StatusNotFound:
		return "Not Found"
	case http.StatusMethodNotAllowed:
		return "Method Not Allowed"
	case http.StatusInternalServerError:
		return "Internal Server Error"
	default:
		return "Error"
	}
}
