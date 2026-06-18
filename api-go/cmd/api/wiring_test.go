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

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/designations"
	"github.com/AmyDe/town-crier/api-go/internal/geocoding"
	"github.com/AmyDe/town-crier/api-go/internal/offercodes"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/savedapplications"
	"github.com/AmyDe/town-crier/api-go/internal/subscriptions"
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

// QueryItemsCrossPartition / QueryPageCrossPartition let fakeItems back a
// profiles.AdminStore; the wiring tests only need the empty result path.
func (f *fakeItems) QueryItemsCrossPartition(_ context.Context, _ string, _ map[string]any) ([][]byte, error) {
	return nil, nil
}

// ReadItemWithETag returns the item body and a synthetic etag, satisfying the
// offercodes.cosmosItems CAS interface.
func (f *fakeItems) ReadItemWithETag(_ context.Context, _, id string) ([]byte, string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	raw, ok := f.items[id]
	if !ok {
		return nil, "", false, nil
	}
	return raw, "etag-" + id, true, nil
}

// ReplaceItemWithETag replaces the item unconditionally (no real etag enforcement
// needed in wiring tests).
func (f *fakeItems) ReplaceItemWithETag(_ context.Context, _ string, id string, item []byte, _ string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items[id] = item
	return "etag-replaced-" + id, nil
}

func (f *fakeItems) QueryPageCrossPartition(_ context.Context, _ string, _ map[string]any, _ int, _ string) ([][]byte, string, error) {
	return nil, "", nil
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

// testGeocodeClient and testDesignationClient point at an unroutable address; the
// deny-all wiring tests never reach the handlers, so the upstream is never
// called.
func testGeocodeClient(t *testing.T) *geocoding.Client {
	t.Helper()
	c, err := geocoding.NewClient("http://127.0.0.1:0", http.DefaultClient)
	if err != nil {
		t.Fatalf("geocoding.NewClient: %v", err)
	}
	return c
}

func testDesignationClient(t *testing.T) *designations.Client {
	t.Helper()
	c, err := designations.NewClient("http://127.0.0.1:0", http.DefaultClient)
	if err != nil {
		t.Fatalf("designations.NewClient: %v", err)
	}
	return c
}

// testGeocodeClientWith and testDesignationClientWith build clients pointing at
// a specific upstream (used in integration-style wiring tests).
func testGeocodeClientWith(t *testing.T, baseURL string, httpClient *http.Client) *geocoding.Client {
	t.Helper()
	c, err := geocoding.NewClient(baseURL, httpClient)
	if err != nil {
		t.Fatalf("geocoding.NewClient(%q): %v", baseURL, err)
	}
	return c
}

func testDesignationClientWith(t *testing.T, baseURL string, httpClient *http.Client) *designations.Client {
	t.Helper()
	c, err := designations.NewClient(baseURL, httpClient)
	if err != nil {
		t.Fatalf("designations.NewClient(%q): %v", baseURL, err)
	}
	return c
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	return newRouter(denyAllValidator{}, []string{"https://towncrierapp.uk"}, nil, profiles.NoOpAuth0Client{}, "", profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, nil, nil, nil, testGeocodeClient(t), testDesignationClient(t), nil, nil, "", nil, nil, "", nil, nil, slog.New(slog.DiscardHandler))
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
		// Geocode and designations are authed, not anonymous: no token -> 401.
		{"geocode without token", http.MethodGet, "/v1/geocode/SW1A1AA"},
		{"designations without token", http.MethodGet, "/v1/designations?latitude=55&longitude=2"},
		// Offer-code redeem is authed: no token -> 401.
		{"redeem without token", http.MethodPost, "/v1/offer-codes/redeem"},
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
	appStore := applications.NewCosmosStore(newFakeItems())
	savedStore := savedapplications.NewCosmosStore(newFakeItems())
	validator := staticValidator{claims: auth.Claims{Subject: "auth0|wiretest", Email: "wire@example.com", EmailVerified: true}}
	h := newRouter(validator, []string{"https://towncrierapp.uk"}, store, profiles.NoOpAuth0Client{}, "", profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, watchZoneStore, appStore, savedStore, testGeocodeClient(t), testDesignationClient(t), nil, nil, "", nil, nil, "", nil, nil, logger)

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

	// Saved-application routes are wired behind the same chain (empty list array).
	rec = serveReq(t, h, http.MethodGet, "/v1/me/saved-applications", "", "Bearer tok")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/me/saved-applications status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `[]` {
		t.Errorf("GET /v1/me/saved-applications body = %s, want []", got)
	}

	// application-authorities is wired off the watch-zone store (empty set).
	rec = serveReq(t, h, http.MethodGet, "/v1/me/application-authorities", "", "Bearer tok")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/me/application-authorities status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"authorities":[],"count":0}` {
		t.Errorf("GET /v1/me/application-authorities body = %s", got)
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

// TestRouter_GeocodeAndDesignationsDispatch proves an authenticated request
// reaches the geocode and designation handlers (not just the auth fallback). A
// single stub upstream backs both outbound clients, routing by path.
func TestRouter_GeocodeAndDesignationsDispatch(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/postcodes/"):
			_, _ = w.Write([]byte(`{"status":200,"result":{"latitude":51.5,"longitude":-0.14}}`))
		case r.URL.Path == "/api/v1/entity.json":
			// No intersecting entity -> the none context.
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(upstream.Close)

	logger := slog.New(slog.DiscardHandler)
	validator := staticValidator{claims: auth.Claims{Subject: "auth0|wiretest", Email: "wire@example.com", EmailVerified: true}}
	geocodeClient := testGeocodeClientWith(t, upstream.URL, upstream.Client())
	designationClient := testDesignationClientWith(t, upstream.URL, upstream.Client())
	h := newRouter(validator, []string{"https://towncrierapp.uk"}, nil, profiles.NoOpAuth0Client{}, "", profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, nil, nil, nil, geocodeClient, designationClient, nil, nil, "", nil, nil, "", nil, nil, logger)

	rec := serveReq(t, h, http.MethodGet, "/v1/geocode/SW1A%201AA", "", "Bearer tok")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/geocode status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != `{"coordinates":{"latitude":51.5,"longitude":-0.14}}` {
		t.Errorf("geocode body = %s", got)
	}

	rec = serveReq(t, h, http.MethodGet, "/v1/designations?latitude=55&longitude=2", "", "Bearer tok")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/designations status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != `{"isWithinConservationArea":false,"conservationAreaName":null,"isWithinListedBuildingCurtilage":false,"listedBuildingGrade":null,"isWithinArticle4Area":false}` {
		t.Errorf("designations body = %s", got)
	}
}

// TestRouter_SubscriptionsWired confirms the verify endpoint is authed and the
// App Store webhook is anonymous to Auth0 (the signed JWS is its auth). A
// deny-all validator is used: the webhook still reaches its handler (a malformed
// body returns 400 malformed_request), while verify falls to the 401 fallback.
func TestRouter_SubscriptionsWired(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	store := profiles.NewCosmosStore(newFakeItems())
	adminStore := profiles.NewAdminStore(newFakeItems())
	notifStore := subscriptions.NewCosmosNotificationStore(newFakeItems(), time.Now)
	roots, err := subscriptions.LoadAppleRootCertificates()
	if err != nil {
		t.Fatalf("LoadAppleRootCertificates: %v", err)
	}
	verifier, err := subscriptions.NewJWSVerifier(roots, time.Now)
	if err != nil {
		t.Fatalf("NewJWSVerifier: %v", err)
	}

	h := newRouter(denyAllValidator{}, []string{"https://towncrierapp.uk"}, store, profiles.NoOpAuth0Client{}, "", profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, nil, nil, nil, testGeocodeClient(t), testDesignationClient(t), nil, adminStore, "", verifier, notifStore, "uk.towncrierapp.mobile", []string{"Production"}, nil, logger)

	// Webhook is anonymous: a malformed body reaches the handler -> 400 with the
	// malformed_request envelope, not the WWW-Authenticate 401 fallback.
	rec := serveReq(t, h, http.MethodPost, "/v1/webhooks/appstore", "{not json", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("webhook status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "malformed_request") {
		t.Errorf("webhook body = %s, want malformed_request", rec.Body.String())
	}

	// Verify is authed: with no token the deny-all fallback returns 401.
	rec = serveReq(t, h, http.MethodPost, "/v1/subscriptions/verify", `{"signedTransaction":"x"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("verify status = %d, want 401", rec.Code)
	}
}

// TestRouter_AdminGate confirms the admin routes are wired, anonymous to Auth0,
// and gated solely by the X-Admin-Key. A deny-all validator is used: if the
// routes were behind the Auth0 fallback they would 401 with WWW-Authenticate:
// Bearer; the admin gate's 401 carries no such header, and a correct key reaches
// the handler.
func TestRouter_AdminGate(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	offerStore := offercodes.NewCosmosStore(newFakeItems())
	adminStore := profiles.NewAdminStore(newFakeItems())
	h := newRouter(denyAllValidator{}, []string{"https://towncrierapp.uk"}, nil, profiles.NoOpAuth0Client{}, "", profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, nil, nil, nil, testGeocodeClient(t), testDesignationClient(t), offerStore, adminStore, "s3cret", nil, nil, "", nil, nil, logger)

	// No key: the admin gate rejects with a bodyless 401 and NO WWW-Authenticate
	// (distinguishing it from the Auth0 fallback-deny).
	noKey := adminRequest(t, h, "")
	if noKey.Code != http.StatusUnauthorized {
		t.Fatalf("no key: status = %d, want 401", noKey.Code)
	}
	if got := noKey.Header().Get("WWW-Authenticate"); got != "" {
		t.Errorf("no key: WWW-Authenticate = %q, want empty (admin gate, not Auth0)", got)
	}

	// Correct key: the request reaches the list handler and returns the empty page.
	withKey := adminRequest(t, h, "s3cret")
	if withKey.Code != http.StatusOK {
		t.Fatalf("with key: status = %d body = %s", withKey.Code, withKey.Body.String())
	}
	if got := withKey.Body.String(); got != `{"items":[],"continuationToken":null}` {
		t.Errorf("with key: body = %s", got)
	}
}

func adminRequest(t *testing.T, h http.Handler, key string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/admin/users", nil)
	if key != "" {
		req.Header.Set("X-Admin-Key", key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
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
