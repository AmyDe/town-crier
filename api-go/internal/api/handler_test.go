package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
)

// TestMe_ReturnsSubjectAsUserId pins the .NET GET /api/me contract:
// Results.Ok(new UserIdResponse(sub)) -> {"userId":"<sub>"} (camelCase via the
// record's explicit JsonPropertyName), Content-Type application/json with the
// charset that ASP.NET's Results.Ok writes.
func TestMe_ReturnsSubjectAsUserId(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	Routes(mux, slog.New(slog.DiscardHandler))

	// The auth middleware would normally inject the subject; here we inject it
	// directly so the handler is tested in isolation.
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req = req.WithContext(auth.WithSubject(context.Background(), "auth0|abc123"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("content-type = %q, want %q", got, "application/json; charset=utf-8")
	}
	if got := rec.Body.String(); got != `{"userId":"auth0|abc123"}` {
		t.Errorf("body = %q, want %q", got, `{"userId":"auth0|abc123"}`)
	}
}

// TestMe_EmptySubjectStillServes documents the boundary: the route is reached
// only after the auth middleware authenticated the request, so the subject is
// always non-empty in practice. The handler does no extra guarding — it echoes
// whatever subject the middleware injected — matching .NET's `sub` read.
func TestMe_EchoesInjectedSubject(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	Routes(mux, slog.New(slog.DiscardHandler))

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req = req.WithContext(auth.WithSubject(context.Background(), "google-oauth2|999"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if got := rec.Body.String(); got != `{"userId":"google-oauth2|999"}` {
		t.Errorf("body = %q, want subject echoed", got)
	}
}
