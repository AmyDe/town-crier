package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// okHandler is a trivial terminal handler used to verify CORS headers are
// layered onto a normal response.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func TestCORS_AllowedOriginEchoed(t *testing.T) {
	t.Parallel()

	h := CORS([]string{"https://towncrierapp.uk", "http://localhost:5173"})(okHandler())

	tests := []struct {
		name   string
		origin string
	}{
		{"prod origin", "https://towncrierapp.uk"},
		{"dev origin", "http://localhost:5173"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/me", nil)
			req.Header.Set("Origin", tc.origin)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tc.origin {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, tc.origin)
			}
			// ASP.NET echoes the matched origin and adds Vary: Origin so caches
			// don't serve one origin's CORS response to another.
			if got := rec.Header().Get("Vary"); got != "Origin" {
				t.Errorf("Vary = %q, want %q", got, "Origin")
			}
		})
	}
}

func TestCORS_DisallowedOriginNotEchoed(t *testing.T) {
	t.Parallel()

	h := CORS([]string{"https://towncrierapp.uk"})(okHandler())

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/me", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty for disallowed origin", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (request still served; browser enforces CORS)", rec.Code)
	}
}

func TestCORS_NoOriginHeaderUntouched(t *testing.T) {
	t.Parallel()

	h := CORS([]string{"https://towncrierapp.uk"})(okHandler())

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/me", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty when no Origin header", got)
	}
}

func TestCORS_PreflightShortCircuits(t *testing.T) {
	t.Parallel()

	// A terminal handler that records whether it was reached. A CORS preflight
	// (OPTIONS with Access-Control-Request-Method) must be answered by the CORS
	// layer and never reach the application handler.
	reached := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})
	h := CORS([]string{"https://towncrierapp.uk"})(next)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "/api/me", nil)
	req.Header.Set("Origin", "https://towncrierapp.uk")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if reached {
		t.Error("preflight reached application handler; CORS layer must short-circuit it")
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://towncrierapp.uk" {
		t.Errorf("Access-Control-Allow-Origin = %q, want echoed origin", got)
	}
	// AllowAnyHeader / AllowAnyMethod: echo what the browser asked for.
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET" {
		t.Errorf("Access-Control-Allow-Methods = %q, want echoed request method", got)
	}
}

func TestCORS_PreflightEchoesRequestedHeaders(t *testing.T) {
	t.Parallel()

	h := CORS([]string{"https://towncrierapp.uk"})(okHandler())

	req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "/api/me", nil)
	req.Header.Set("Origin", "https://towncrierapp.uk")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Authorization, Content-Type" {
		t.Errorf("Access-Control-Allow-Headers = %q, want echoed requested headers", got)
	}
}
