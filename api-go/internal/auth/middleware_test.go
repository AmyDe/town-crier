package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeValidator is a hand-written test double for the consumer-side
// TokenValidator interface. No JWKS network call happens in unit tests: the
// fake maps known token strings to claims and returns an error otherwise.
type fakeValidator struct {
	subjectByToken map[string]string
	claimsByToken  map[string]Claims
}

func (f *fakeValidator) ValidateToken(_ context.Context, token string) (Claims, error) {
	if c, ok := f.claimsByToken[token]; ok {
		return c, nil
	}
	if sub, ok := f.subjectByToken[token]; ok {
		return Claims{Subject: sub}, nil
	}
	return Claims{}, errors.New("invalid token")
}

// muxWith builds a mux with one anonymous route and one authenticated route so
// the middleware's three branches (anonymous / authenticated / unmatched) are
// all exercised against a realistic pattern set.
func muxWith(t *testing.T) (*http.ServeMux, map[string]struct{}) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"Healthy"}`))
	})
	mux.HandleFunc("GET /api/me", func(w http.ResponseWriter, r *http.Request) {
		// Echo the subject so the test can confirm claims reached the handler.
		_, _ = w.Write([]byte(Subject(r.Context())))
	})
	anonymous := map[string]struct{}{"GET /v1/health": {}}
	return mux, anonymous
}

func TestRequireAuth_AnonymousRouteServedWithoutToken(t *testing.T) {
	t.Parallel()

	mux, anonymous := muxWith(t)
	v := &fakeValidator{}
	h := RequireAuth(v, mux, anonymous)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != `{"status":"Healthy"}` {
		t.Errorf("body = %q, want health body", got)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got != "" {
		t.Errorf("WWW-Authenticate = %q, want empty on anonymous route", got)
	}
}

func TestRequireAuth_AuthenticatedRouteRejectsMissingToken(t *testing.T) {
	t.Parallel()

	mux, anonymous := muxWith(t)
	v := &fakeValidator{}
	h := RequireAuth(v, mux, anonymous)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/me", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assertChallenge(t, rec)
}

func TestRequireAuth_AuthenticatedRouteRejectsInvalidToken(t *testing.T) {
	t.Parallel()

	mux, anonymous := muxWith(t)
	v := &fakeValidator{subjectByToken: map[string]string{"good": "auth0|abc"}}
	h := RequireAuth(v, mux, anonymous)

	tests := []struct {
		name   string
		header string
	}{
		{"wrong token", "Bearer bad"},
		{"not bearer scheme", "Basic good"},
		{"empty bearer", "Bearer "},
		{"malformed header", "good"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/me", nil)
			req.Header.Set("Authorization", tc.header)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			assertChallenge(t, rec)
		})
	}
}

func TestRequireAuth_AuthenticatedRouteAcceptsValidToken(t *testing.T) {
	t.Parallel()

	mux, anonymous := muxWith(t)
	v := &fakeValidator{subjectByToken: map[string]string{"good": "auth0|abc123"}}
	h := RequireAuth(v, mux, anonymous)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer good")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "auth0|abc123" {
		t.Errorf("subject in handler = %q, want auth0|abc123", got)
	}
}

func TestRequireAuth_ThreadsFullClaimsToHandler(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/me", func(w http.ResponseWriter, r *http.Request) {
		c := ClaimsFrom(r.Context())
		// Confirm every claim the create-profile path needs reached the handler.
		_, _ = w.Write([]byte(c.Subject + "|" + c.Email + "|" + boolStr(c.EmailVerified) + "|" + c.SubscriptionTier))
	})
	anonymous := map[string]struct{}{}
	v := &fakeValidator{claimsByToken: map[string]Claims{
		"good": {Subject: "auth0|abc", Email: "u@example.com", EmailVerified: true, SubscriptionTier: "Pro"},
	}}
	h := RequireAuth(v, mux, anonymous)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer good")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got, want := rec.Body.String(), "auth0|abc|u@example.com|true|Pro"; got != want {
		t.Errorf("claims in handler = %q, want %q", got, want)
	}
	// Subject helper still works, derived from the threaded claims.
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func TestRequireAuth_UnmatchedRouteFallsToDeny(t *testing.T) {
	t.Parallel()

	mux, anonymous := muxWith(t)
	v := &fakeValidator{subjectByToken: map[string]string{"good": "auth0|abc"}}
	h := RequireAuth(v, mux, anonymous)

	// Unmatched paths (incl. "/") and method mismatches return the fallback-deny
	// 401 even with a valid token — there is no endpoint to authorise against,
	// mirroring .NET's no-endpoint -> fallback policy.
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"root", http.MethodGet, "/"},
		{"unknown path", http.MethodGet, "/v1/does-not-exist"},
		{"wrong method on known path", http.MethodPost, "/api/me"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer good")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			assertChallenge(t, rec)
		})
	}
}

// assertChallenge verifies the fallback-deny contract: 401, bodyless (the
// PascalCase envelope is added downstream by middleware.ErrorBody), and the
// WWW-Authenticate: Bearer header .NET's JwtBearer handler emits on challenge.
func assertChallenge(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Errorf("WWW-Authenticate = %q, want %q", got, "Bearer")
	}
	if got := rec.Body.String(); got != "" {
		t.Errorf("body = %q, want empty (envelope backfilled by ErrorBody)", got)
	}
}
