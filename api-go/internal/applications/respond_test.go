package applications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func pgTimeoutError(t *testing.T) error {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		if c, err := ln.Accept(); err == nil {
			time.Sleep(time.Second)
			_ = c.Close()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = pgconn.Connect(ctx, "postgres://u@"+ln.Addr().String()+"/db?sslmode=disable")
	if !pgconn.Timeout(err) {
		t.Fatalf("expected a pgconn timeout, got %v", err)
	}
	return err
}

func TestServerError_Status(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"deadline exceeded", fmt.Errorf("search: %w", context.DeadlineExceeded), http.StatusServiceUnavailable},
		{"pg timeout", pgTimeoutError(t), http.StatusServiceUnavailable},
		{"generic error", errors.New("boom"), http.StatusInternalServerError},
		{"canceled writes nothing", fmt.Errorf("search: %w", context.Canceled), http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			logger := slog.New(slog.DiscardHandler)

			serverError(rec, r, logger, "test", tt.err)

			if rec.Code != tt.want {
				t.Errorf("status: got %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

var errStoreFailed = errors.New("store failed")
