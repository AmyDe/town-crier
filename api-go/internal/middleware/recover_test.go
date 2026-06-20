package middleware

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// logSpy captures slog records for test assertions.
type logSpy struct {
	mu      sync.Mutex
	records []slog.Record
}

func (s *logSpy) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (s *logSpy) Handle(_ context.Context, r slog.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, r.Clone())
	return nil
}

func (s *logSpy) WithAttrs(_ []slog.Attr) slog.Handler { return s }
func (s *logSpy) WithGroup(_ string) slog.Handler      { return s }

// attrString returns the string representation of the first attribute matching
// key across all captured records, or empty string when absent.
func (s *logSpy) attrString(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.records {
		var found string
		var ok bool
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == key {
				found = a.Value.String()
				ok = true
				return false
			}
			return true
		})
		if ok {
			return found
		}
	}
	return ""
}

// TestRecover_PanicBecomes500WithGenericDetail pins the security fix (GH#516):
// a panic in a downstream handler must produce a 500 whose Detail is the
// generic constant panicDetail — never the runtime/internal panic text.
// Recover runs INSIDE ErrorBody so the envelope is written by the same backfill
// path that produces the bodyless 4xx envelopes.
func TestRecover_PanicBecomes500WithGenericDetail(t *testing.T) {
	t.Parallel()

	spy := &logSpy{}
	status, contentType, body := serveChainWithLogger(t, slog.New(spy), func(http.ResponseWriter, *http.Request) {
		panic("boom: handler exploded")
	})

	if status != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", status, http.StatusInternalServerError)
	}
	if contentType != "application/json" {
		t.Errorf("content-type: got %q, want %q", contentType, "application/json")
	}
	// Client body must contain the generic detail, not the panic text.
	want := `{"Status":500,"Title":"Internal Server Error","Detail":"` + panicDetail + `"}`
	if body != want {
		t.Errorf("body: got %s, want %s", body, want)
	}
	// The full panic text must still be logged server-side for debugging.
	if got := spy.attrString("error"); got != "boom: handler exploded" {
		t.Errorf("logged error: got %q, want %q", got, "boom: handler exploded")
	}
}

// TestRecover_PanicWithErrorValueUsesGenericDetail covers panicking with an
// error value — the client still sees the generic Detail, not the error text.
func TestRecover_PanicWithErrorValueUsesGenericDetail(t *testing.T) {
	t.Parallel()

	spy := &logSpy{}
	status, _, body := serveChainWithLogger(t, slog.New(spy), func(http.ResponseWriter, *http.Request) {
		panic(context.DeadlineExceeded)
	})

	if status != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", status, http.StatusInternalServerError)
	}
	want := `{"Status":500,"Title":"Internal Server Error","Detail":"` + panicDetail + `"}`
	if body != want {
		t.Errorf("body: got %s, want %s", body, want)
	}
	// The full deadline exceeded error must still be logged server-side.
	if got := spy.attrString("error"); got != "context deadline exceeded" {
		t.Errorf("logged error: got %q, want %q", got, "context deadline exceeded")
	}
}

// TestRecover_NoPanicPassesThrough confirms the happy path is untouched: a
// normal 200 response flows through both Recover and ErrorBody unchanged.
func TestRecover_NoPanicPassesThrough(t *testing.T) {
	t.Parallel()

	status, contentType, body := serveChain(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"userId":"auth0|abc"}`))
	})

	if status != http.StatusOK {
		t.Errorf("status: got %d, want %d", status, http.StatusOK)
	}
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("content-type: got %q, want %q", contentType, "application/json; charset=utf-8")
	}
	if want := `{"userId":"auth0|abc"}`; body != want {
		t.Errorf("body: got %s, want %s", body, want)
	}
}

// TestRecover_PanicAfterPartialWriteDoesNotDoubleWrite guards the edge where a
// handler writes some bytes and then panics: the response has already started,
// so we must not attempt to write the 500 envelope on top of it (which would
// corrupt it); the recover is swallowed (logged) and the already-sent status
// stands.
func TestRecover_PanicAfterPartialWriteDoesNotDoubleWrite(t *testing.T) {
	t.Parallel()

	status, _, body := serveChain(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`partial`))
		panic("too late")
	})

	if status != http.StatusOK {
		t.Errorf("status: got %d, want %d (response already started)", status, http.StatusOK)
	}
	if body != "partial" {
		t.Errorf("body: got %q, want %q", body, "partial")
	}
}

// serveChain runs a request through ErrorBody wrapping Recover wrapping the
// handler — the production ordering for the panic-to-500 path. Uses a discard
// logger; use serveChainWithLogger when log assertions are needed.
func serveChain(t *testing.T, next http.HandlerFunc) (status int, contentType string, body string) {
	t.Helper()
	return serveChainWithLogger(t, slog.New(slog.DiscardHandler), next)
}

// serveChainWithLogger is like serveChain but accepts an explicit logger,
// allowing callers to pass a logSpy for server-side log assertions.
func serveChainWithLogger(t *testing.T, logger *slog.Logger, next http.HandlerFunc) (status int, contentType string, body string) {
	t.Helper()

	chain := ErrorBody(logger)(Recover(logger)(next))
	srv := httptest.NewServer(chain)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, resp.Header.Get("Content-Type"), string(raw)
}
