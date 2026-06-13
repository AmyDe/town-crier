package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// denyAllValidator is the validator the API runs with when Auth0 config is
// absent: every token is rejected, so authenticated routes return the
// fallback-deny 401 — exactly what the contract tests assert.
type denyAllValidator struct{}

func (denyAllValidator) ValidateToken(context.Context, string) (auth.Claims, error) {
	return auth.Claims{}, context.Canceled // any non-nil error denies
}

// staticValidator accepts every token as the given claims, letting wiring tests
// exercise the authenticated pipeline without JWKS.
type staticValidator struct{ claims auth.Claims }

func (v staticValidator) ValidateToken(context.Context, string) (auth.Claims, error) {
	return v.claims, nil
}

// fakeItems is an in-memory CosmosItems so wiring tests run the real store,
// handlers, and post-auth middlewares with no Cosmos dependency.
type fakeItems struct {
	mu    sync.Mutex
	items map[string][]byte
}

func newFakeItems() *fakeItems { return &fakeItems{items: map[string][]byte{}} }

func (f *fakeItems) ReadItem(_ context.Context, _, id string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	raw, ok := f.items[id]
	if !ok {
		return nil, notFoundErr()
	}
	return raw, nil
}

func (f *fakeItems) UpsertItem(_ context.Context, _ string, item []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	// The document's id field is the user id; the store always writes id == partition key.
	f.items[idFromDoc(item)] = item
	return nil
}

func (f *fakeItems) DeleteItem(_ context.Context, _, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.items[id]; !ok {
		return notFoundErr()
	}
	delete(f.items, id)
	return nil
}

// QueryItems lets fakeItems also back a watchzones store; the wiring tests only
// need the empty-list path, so it returns no documents.
func (f *fakeItems) QueryItems(_ context.Context, _, _ string, _ map[string]any) ([][]byte, error) {
	return nil, nil
}

// notFoundErr mimics the azcore 404 the store's isNotFound detects.
func notFoundErr() error {
	return &azcore.ResponseError{StatusCode: http.StatusNotFound}
}

// idFromDoc extracts the "id" property from a stored document without
// round-tripping the whole shape.
func idFromDoc(raw []byte) string {
	var doc struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &doc)
	return doc.ID
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	return newRouter(denyAllValidator{}, []string{"https://towncrierapp.uk"}, nil, profiles.NoOpAuth0Client{}, "", nil, nil, nil, slog.New(slog.DiscardHandler))
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
			rec := serveReq(t, h, http.MethodGet, tc.path, "", "")
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
		{"me without token", http.MethodGet, "/v1/me"},
		{"root", http.MethodGet, "/"},
		{"unknown path", http.MethodGet, "/v1/nope"},
		{"non-int authority id", http.MethodGet, "/v1/authorities/abc"},
		{"wrong method on me", http.MethodPut, "/api/me"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := serveReq(t, h, tc.method, tc.path, "", "")
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
			rec := serveReq(t, h, http.MethodGet, path, "https://towncrierapp.uk", "")
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://towncrierapp.uk" {
				t.Errorf("Access-Control-Allow-Origin = %q, want echoed origin", got)
			}
		})
	}
}

// TestRouter_AuthenticatedPipeline runs the full wired chain with a store: a
// valid token reaches the /v1/me handlers through rate limiting (headers set,
// .NET RateLimit-before-handler order) and activity recording.
func TestRouter_AuthenticatedPipeline(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	store := profiles.NewCosmosStore(newFakeItems())
	watchZoneStore := watchzones.NewCosmosStore(newFakeItems())
	validator := staticValidator{claims: auth.Claims{Subject: "auth0|wiretest", Email: "wire@example.com", EmailVerified: true}}
	h := newRouter(validator, []string{"https://towncrierapp.uk"}, store, profiles.NoOpAuth0Client{}, "", nil, nil, watchZoneStore, logger)

	// Create the profile, then read it back through the same chain.
	rec := serveReq(t, h, http.MethodPost, "/v1/me", "", "Bearer tok")
	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/me status = %d body = %s", rec.Code, rec.Body.String())
	}

	rec = serveReq(t, h, http.MethodGet, "/v1/me", "", "Bearer tok")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/me status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"userId":"auth0|wiretest"`) {
		t.Errorf("GET /v1/me body missing userId: %s", rec.Body.String())
	}
	if got := rec.Header().Get("X-RateLimit-Limit"); got != "60" {
		t.Errorf("X-RateLimit-Limit = %q, want 60 (free tier)", got)
	}
	if rec.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("X-RateLimit-Remaining missing — rate limiter not in the dispatch path")
	}

	// Watch-zone routes are wired behind the same auth + dispatch chain: a
	// valid token reaches the list handler and gets the empty-array body.
	rec = serveReq(t, h, http.MethodGet, "/v1/me/watch-zones", "", "Bearer tok")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/me/watch-zones status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"zones":[]}` {
		t.Errorf("GET /v1/me/watch-zones body = %s, want {\"zones\":[]}", got)
	}

	// Anonymous routes stay unmetered even on the store-wired router.
	rec = serveReq(t, h, http.MethodGet, "/v1/health", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/health status = %d", rec.Code)
	}
	if got := rec.Header().Get("X-RateLimit-Limit"); got != "" {
		t.Errorf("anonymous route got rate-limit header %q", got)
	}
}

func serveReq(t *testing.T, h http.Handler, method, path, origin, authz string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req := httptest.NewRequestWithContext(ctx, method, path, nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
