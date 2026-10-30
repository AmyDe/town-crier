package platform

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
)

// StatusClientClosedRequest is the nginx convention for a request whose client
// disconnected before the response was ready. It stays below the 5xx alert
// thresholds and, unlike an unwritten response, is not recorded as a 200.
const StatusClientClosedRequest = 499

const sqlStateQueryCanceled = "57014"

// ClassifyRequestError maps a handler error to a response status using the
// request context as the authority. A cancelled request context, or an error
// wrapping context.Canceled, gives 499. A deadline, a driver timeout or a
// PgError 57014 with a live request context gives 503. Any other error gives 500.
func ClassifyRequestError(r *http.Request, err error) int {
	if r.Context().Err() == context.Canceled || errors.Is(err, context.Canceled) {
		return StatusClientClosedRequest
	}
	var pgErr *pgconn.PgError
	if errors.Is(err, context.DeadlineExceeded) || pgconn.Timeout(err) ||
		(errors.As(err, &pgErr) && pgErr.Code == sqlStateQueryCanceled) {
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}

// RespondRequestError writes the bodyless status chosen by ClassifyRequestError.
// The error envelope is backfilled by middleware.ErrorBody. Every status except
// 499 is logged at Error with msg, the error and attrs.
func RespondRequestError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, msg string, err error, attrs ...any) {
	status := ClassifyRequestError(r, err)
	if status != StatusClientClosedRequest {
		logger.ErrorContext(r.Context(), msg, append(attrs, "error", err)...) //nolint:sloglint // msg is a constant at every call site
	}
	w.WriteHeader(status)
}
