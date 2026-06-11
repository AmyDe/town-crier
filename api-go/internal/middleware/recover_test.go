package middleware

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestRecover_PanicBecomes500WithDetail pins the .NET unhandled-exception
// contract (GH#418, ErrorResponseMiddleware): a panic in a downstream handler
// becomes a 500 whose backfilled body carries the panic message in Detail —
// {"Status":500,"Title":"Internal Server Error","Detail":"<message>"}. Recover
// runs INSIDE ErrorBody so the envelope is written by the same backfill path
// that produces the bodyless 4xx envelopes.
func TestRecover_PanicBecomes500WithDetail(t *testing.T) {
	t.Parallel()

	status, contentType, _, body := serveChain(t, func(http.ResponseWriter, *http.Request) {
		panic("boom: handler exploded")
	})

	if status != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", status, http.StatusInternalServerError)
	}
	if contentType != "application/json" {
		t.Errorf("content-type: got %q, want %q", contentType, "application/json")
	}
	if want := `{"Status":500,"Title":"Internal Server Error","Detail":"boom: handler exploded"}`; body != want {
		t.Errorf("body: got %s, want %s", body, want)
	}
}

// TestRecover_PanicWithErrorValuePropagatesMessage covers panicking with an
// error value rather than a string — its Error() text is what .NET's
// ex.Message would carry.
func TestRecover_PanicWithErrorValuePropagatesMessage(t *testing.T) {
	t.Parallel()

	status, _, _, body := serveChain(t, func(http.ResponseWriter, *http.Request) {
		panic(context.DeadlineExceeded)
	})

	if status != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", status, http.StatusInternalServerError)
	}
	if want := `{"Status":500,"Title":"Internal Server Error","Detail":"context deadline exceeded"}`; body != want {
		t.Errorf("body: got %s, want %s", body, want)
	}
}

// TestRecover_NoPanicPassesThrough confirms the happy path is untouched: a
// normal 200 response flows through both Recover and ErrorBody unchanged.
func TestRecover_NoPanicPassesThrough(t *testing.T) {
	t.Parallel()

	status, contentType, _, body := serveChain(t, func(w http.ResponseWriter, _ *http.Request) {
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
// so .NET cannot replace the body. We must not attempt to write the 500
// envelope on top of a started response (which would corrupt it); the recover
// is swallowed (logged) and the already-sent status stands.
func TestRecover_PanicAfterPartialWriteDoesNotDoubleWrite(t *testing.T) {
	t.Parallel()

	status, _, _, body := serveChain(t, func(w http.ResponseWriter, _ *http.Request) {
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
// handler — the production ordering for the panic-to-500 path.
func serveChain(t *testing.T, next http.HandlerFunc) (status int, contentType string, header http.Header, body string) {
	t.Helper()

	logger := slog.New(slog.DiscardHandler)
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
	return resp.StatusCode, resp.Header.Get("Content-Type"), resp.Header, string(raw)
}
