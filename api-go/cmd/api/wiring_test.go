package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// denyAllValidator is the validator the API runs with when Auth0 config is
// absent (the dev Go app today): every token is rejected, so authenticated
// routes return the fallback-deny 401 — exactly what the contract tests assert.
type denyAllValidator struct{}

func (denyAllValidator) ValidateToken(context.Context, string) (string, error) {
	return "", context.Canceled // any non-nil error denies
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	return newRouter(denyAllValidator{}, []string{"https://towncrierapp.uk"}, slog.New(slog.DiscardHandler))
}

// TestRouter_AnonymousRoutesServedWithoutToken confirms the iteration-0/1
// anonymous endpoints still serve once the auth fallback owns the chain.
func TestRouter_AnonymousRoutesServedWithoutToken(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t)
	for _, tc := range []struct {
		path       string
		wantStatus int
	}{
		{"/health", http.StatusOK},
		{"/v1/health", http.StatusOK},
		{"/v1/version-config", http.StatusOK},
		{"/v1/legal/privacy", http.StatusOK},
		{"/v1/legal/unknown", http.StatusNotFound}, // anonymous route, bodyless 404 backfilled
		{"/v1/authorities", http.StatusOK},
		{"/v1/authorities/384", http.StatusOK},
		{"/v1/authorities/99999999", http.StatusNotFound},
	} {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			rec := serveReq(t, h, http.MethodGet, tc.path, "")
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

// TestRouter_FallbackDeny pins the 401 surface: protected routes, unmatched
// paths, the root, and non-int authority ids all return 401 with
// WWW-Authenticate: Bearer and the PascalCase envelope.
func TestRouter_FallbackDeny(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t)
	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{"api me without token", http.MethodGet, "/api/me"},
		{"root", http.MethodGet, "/"},
		{"unknown path", http.MethodGet, "/v1/nope"},
		{"non-int authority id", http.MethodGet, "/v1/authorities/abc"},
		{"wrong method on me", http.MethodPost, "/api/me"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := serveReq(t, h, tc.method, tc.path, "")
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Errorf("WWW-Authenticate = %q, want Bearer", got)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			if want := `{"Status":401,"Title":"Unauthorized","Detail":null}`; rec.Body.String() != want {
				t.Errorf("body = %s, want %s", rec.Body.String(), want)
			}
		})
	}
}

// TestRouter_CorsLayeredOnAllResponses confirms CORS is the outermost layer:
// the matched-origin header appears on a 401 just as on a 200, matching .NET's
// CORS-before-everything ordering.
func TestRouter_CorsLayeredOnAllResponses(t *testing.T) {
	t.Parallel()

	h := newTestHandler(t)
	for _, path := range []string{"/v1/health", "/api/me"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			rec := serveReq(t, h, http.MethodGet, path, "https://towncrierapp.uk")
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://towncrierapp.uk" {
				t.Errorf("Access-Control-Allow-Origin = %q, want echoed origin", got)
			}
		})
	}
}

func serveReq(t *testing.T, h http.Handler, method, path, origin string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req := httptest.NewRequestWithContext(ctx, method, path, nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
