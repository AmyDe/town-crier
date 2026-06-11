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

// The expected bodies are the live .NET dev API's wire bytes (captured
// 2026-06-11, e.g. GET /v1/legal/unknown), not derived from the Go marshaller:
// PascalCase keys in record declaration order, Detail explicitly null, and
// Content-Type exactly "application/json" — no charset, unlike handler-written
// bodies.
func TestErrorBody_BackfillsEmptyErrorResponses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   int
		wantBody string
	}{
		{"400 bad request", http.StatusBadRequest, `{"Status":400,"Title":"Bad Request","Detail":null}`},
		{"401 unauthorized", http.StatusUnauthorized, `{"Status":401,"Title":"Unauthorized","Detail":null}`},
		{"403 forbidden", http.StatusForbidden, `{"Status":403,"Title":"Forbidden","Detail":null}`},
		{"404 not found", http.StatusNotFound, `{"Status":404,"Title":"Not Found","Detail":null}`},
		{"405 method not allowed", http.StatusMethodNotAllowed, `{"Status":405,"Title":"Method Not Allowed","Detail":null}`},
		{"429 falls through the reason map to Error", http.StatusTooManyRequests, `{"Status":429,"Title":"Error","Detail":null}`},
		{"500 internal server error", http.StatusInternalServerError, `{"Status":500,"Title":"Internal Server Error","Detail":null}`},
		{"418 unmapped status uses Error", http.StatusTeapot, `{"Status":418,"Title":"Error","Detail":null}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			status, contentType, _, body := serve(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
			})

			if status != tc.status {
				t.Errorf("status: got %d, want %d", status, tc.status)
			}
			if contentType != "application/json" {
				t.Errorf("content-type: got %q, want %q", contentType, "application/json")
			}
			if body != tc.wantBody {
				t.Errorf("body: got %s, want %s", body, tc.wantBody)
			}
		})
	}
}

func TestErrorBody_PreservesHeadersSetBeforeError(t *testing.T) {
	t.Parallel()

	status, contentType, header, body := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate", "Bearer")
		// A handler-set Content-Type on a bodyless error is overwritten, the
		// way .NET's middleware assigns Response.ContentType unconditionally.
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusUnauthorized)
	})

	if status != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", status, http.StatusUnauthorized)
	}
	if got := header.Get("WWW-Authenticate"); got != "Bearer" {
		t.Errorf("www-authenticate: got %q, want %q", got, "Bearer")
	}
	if contentType != "application/json" {
		t.Errorf("content-type: got %q, want %q", contentType, "application/json")
	}
	if want := `{"Status":401,"Title":"Unauthorized","Detail":null}`; body != want {
		t.Errorf("body: got %s, want %s", body, want)
	}
}

func TestErrorBody_LeavesOtherResponsesUntouched(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		handler         http.HandlerFunc
		wantStatus      int
		wantContentType string
		wantBody        string
	}{
		{
			name: "200 with body",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				_, _ = w.Write([]byte(`{"status":"Healthy"}`))
			},
			wantStatus:      http.StatusOK,
			wantContentType: "application/json; charset=utf-8",
			wantBody:        `{"status":"Healthy"}`,
		},
		{
			name:            "implicit 200 with empty body",
			handler:         func(http.ResponseWriter, *http.Request) {},
			wantStatus:      http.StatusOK,
			wantContentType: "",
			wantBody:        "",
		},
		{
			name: "204 no content",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			},
			wantStatus:      http.StatusNoContent,
			wantContentType: "",
			wantBody:        "",
		},
		{
			name: "error with a body already written",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"Watch zone quota exceeded"}`))
			},
			wantStatus:      http.StatusForbidden,
			wantContentType: "application/json; charset=utf-8",
			wantBody:        `{"error":"Watch zone quota exceeded"}`,
		},
		{
			name: "error body written across multiple writes",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":`))
				_, _ = w.Write([]byte(`"bad"}`))
			},
			wantStatus: http.StatusBadRequest,
			// net/http sniffs a Content-Type when none is set and a body is
			// written — with or without this middleware — so untouched means
			// the sniffed value, not an absent header.
			wantContentType: "text/plain; charset=utf-8",
			wantBody:        `{"error":"bad"}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			status, contentType, _, body := serve(t, tc.handler)

			if status != tc.wantStatus {
				t.Errorf("status: got %d, want %d", status, tc.wantStatus)
			}
			if contentType != tc.wantContentType {
				t.Errorf("content-type: got %q, want %q", contentType, tc.wantContentType)
			}
			if body != tc.wantBody {
				t.Errorf("body: got %s, want %s", body, tc.wantBody)
			}
		})
	}
}

// serve runs one request through ErrorBody on a real server so header flushing
// behaves as it does in production, returning the observed response.
func serve(t *testing.T, next http.HandlerFunc) (status int, contentType string, header http.Header, body string) {
	t.Helper()

	logger := slog.New(slog.DiscardHandler)
	srv := httptest.NewServer(ErrorBody(logger)(next))
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
