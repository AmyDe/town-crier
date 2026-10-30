package platform

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestClassifyRequestError(t *testing.T) {
	t.Parallel()

	pg57014 := fmt.Errorf("query: %w", &pgconn.PgError{Code: "57014"})
	tests := []struct {
		name      string
		cancelled bool
		err       error
		want      int
	}{
		{"cancelled ctx with 57014", true, pg57014, StatusClientClosedRequest},
		{"cancelled ctx with context.Canceled", true, fmt.Errorf("x: %w", context.Canceled), StatusClientClosedRequest},
		{"cancelled ctx with unrelated error", true, errors.New("boom"), StatusClientClosedRequest},
		{"live ctx with context.Canceled", false, fmt.Errorf("x: %w", context.Canceled), StatusClientClosedRequest},
		{"live ctx with 57014", false, pg57014, http.StatusServiceUnavailable},
		{"live ctx with deadline exceeded", false, fmt.Errorf("x: %w", context.DeadlineExceeded), http.StatusServiceUnavailable},
		{"live ctx with generic error", false, errors.New("boom"), http.StatusInternalServerError},
		{"live ctx with other PgError", false, &pgconn.PgError{Code: "23505"}, http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			if tt.cancelled {
				cancel()
			}
			defer cancel()
			r := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			if got := ClassifyRequestError(r, tt.err); got != tt.want {
				t.Errorf("status = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRespondRequestError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cancelled  bool
		err        error
		wantStatus int
		wantLog    bool
	}{
		{"cancelled is 499 and silent", true, &pgconn.PgError{Code: "57014"}, StatusClientClosedRequest, false},
		{"timeout is 503 and logged", false, &pgconn.PgError{Code: "57014"}, http.StatusServiceUnavailable, true},
		{"other is 500 and logged", false, errors.New("boom"), http.StatusInternalServerError, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			if tt.cancelled {
				cancel()
			}
			defer cancel()
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, nil))
			r := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			RespondRequestError(rec, r, logger, "thing failed", tt.err, "op", "x")

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("body = %q, want empty", rec.Body.String())
			}
			logged := strings.Contains(buf.String(), "level=ERROR") && strings.Contains(buf.String(), "thing failed") && strings.Contains(buf.String(), "op=x")
			if logged != tt.wantLog {
				t.Errorf("error logged = %v, want %v (log: %q)", logged, tt.wantLog, buf.String())
			}
		})
	}
}
