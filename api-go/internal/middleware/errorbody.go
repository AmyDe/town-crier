// Package middleware holds the cross-cutting http.Handler wrappers that
// replicate the .NET API's pipeline behaviours (GH#418). Composition is plain
// func(http.Handler) http.Handler, chained by hand in cmd/api/main.go.
package middleware

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

// errorResponse mirrors the .NET ErrorResponse record: PascalCase keys in
// declaration order, Detail serialized as an explicit null. Detail is always
// nil in this iteration — .NET only populates it from the unhandled-exception
// path, whose Go equivalent (panic recovery) ships in iteration 2.
type errorResponse struct {
	Status int     `json:"Status"`
	Title  string  `json:"Title"`
	Detail *string `json:"Detail"`
}

// ErrorBody replicates the backfill half of the .NET ErrorResponseMiddleware
// contract (GH#418, parity behaviour 1): any response with status >= 400 that
// would otherwise be sent with an empty body gets the PascalCase JSON envelope
// {"Status":<n>,"Title":"<reason>","Detail":null} with Content-Type exactly
// "application/json" (no charset on this path, unlike handler-written bodies).
// Responses that already carry a body pass through untouched — which means the
// ServeMux's own text-bodied 404/405 defaults still diverge from .NET until
// iteration 2's auth fallback owns the unmatched-route surface.
func ErrorBody(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bw := &backfillWriter{ResponseWriter: w}
			next.ServeHTTP(bw, r)
			if err := bw.backfill(); err != nil {
				logger.ErrorContext(r.Context(), "write error body", "status", bw.status, "error", err)
			}
		})
	}
}

// backfillWriter defers WriteHeader for >= 400 statuses until the handler
// either writes body bytes or returns. net/http flushes headers on WriteHeader
// — unlike ASP.NET, where a response only starts on its first body write — so
// deferring is what makes the .NET "response has not started" check (and a
// late Content-Type overwrite) possible.
type backfillWriter struct {
	http.ResponseWriter
	status    int  // deferred status; 0 = WriteHeader not called yet
	flushed   bool // header forwarded to the underlying writer
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
// without a body. Mirrors .NET's status/HasStarted/ContentLength==0 guard.
func (b *backfillWriter) backfill() error {
	if b.flushed || b.status < 400 || b.wroteBody {
		return nil
	}
	body, err := json.Marshal(errorResponse{Status: b.status, Title: reasonPhrase(b.status)})
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

// reasonPhrase mirrors the .NET middleware's GetReasonPhrase switch exactly.
// 429 is deliberately absent from the map there, so it — like every other
// unmapped status — falls through to "Error".
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
