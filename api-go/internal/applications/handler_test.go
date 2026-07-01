package applications

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
)

type fakeAppStore struct {
	app   PlanningApplication
	found bool
	err   error

	lastAuthorityCode string
	lastName          string
}

func (f *fakeAppStore) GetByAuthorityAndName(_ context.Context, authorityCode, name string) (PlanningApplication, bool, error) {
	f.lastAuthorityCode = authorityCode
	f.lastName = name
	return f.app, f.found, f.err
}

type refreshCall struct {
	userID string
	app    PlanningApplication
}

type fakeRefresher struct {
	calls []refreshCall
	err   error
}

func (f *fakeRefresher) RefreshSnapshot(_ context.Context, userID string, app PlanningApplication) error {
	f.calls = append(f.calls, refreshCall{userID: userID, app: app})
	return f.err
}

// fakeResolver is a hand-written authoritySlugResolver. testResolver maps the
// City of London test application's AreaID (471) both ways.
type fakeResolver struct {
	slugToID map[string]int
	idToSlug map[int]string
}

func (f *fakeResolver) SlugToAreaID(slug string) (int, bool) {
	id, ok := f.slugToID[slug]
	return id, ok
}

func (f *fakeResolver) SlugForAreaID(id int) (string, bool) {
	s, ok := f.idToSlug[id]
	return s, ok
}

func testResolver() *fakeResolver {
	return &fakeResolver{
		slugToID: map[string]int{"city-of-london": 471},
		idToSlug: map[int]string{471: "city-of-london"},
	}
}

// serveGet drives the read endpoint with a refresher absent (the nil-safe path)
// and an authenticated subject.
func serveGet(t *testing.T, store appStore, path string) *httptest.ResponseRecorder {
	t.Helper()
	return serveGetWith(t, store, nil, testResolver(), "auth0|u", path)
}

func serveGetWith(t *testing.T, store appStore, refresher snapshotRefresher, resolver authoritySlugResolver, subject, path string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	Routes(mux, store, refresher, resolver, slog.New(slog.DiscardHandler))
	ctx := context.Background()
	if subject != "" {
		ctx = auth.WithSubject(ctx, subject)
	}
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestHandler_GetByAuthorityAndName_Found(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	store := &fakeAppStore{app: a, found: true}

	rec := serveGet(t, store, "/v1/applications/471/24/0123/FUL")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type: got %q", ct)
	}
	// The {name...} wildcard captures the slash-bearing case reference whole.
	if store.lastAuthorityCode != "471" || store.lastName != "24/0123/FUL" {
		t.Errorf("routing: authorityCode=%q name=%q", store.lastAuthorityCode, store.lastName)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["uid"] != a.UID || got["areaId"].(float64) != float64(a.AreaID) {
		t.Errorf("body: %+v", got)
	}
	// Flat coordinates on the wire (no GeoJSON) and an explicit null unread event.
	if got["longitude"].(float64) != *a.Longitude {
		t.Errorf("longitude: got %v", got["longitude"])
	}
	if v, ok := got["latestUnreadEvent"]; !ok || v != nil {
		t.Errorf("latestUnreadEvent must be present and null: %v (present=%v)", v, ok)
	}
	// The by-id detail response now carries the additive authoritySlug, computed
	// round-trip-safe from the resolver (AreaID 471 -> "city-of-london").
	if got["authoritySlug"] != "city-of-london" {
		t.Errorf("authoritySlug: got %v, want city-of-london", got["authoritySlug"])
	}
}

func TestHandler_GetByAuthorityAndName_NotFound(t *testing.T) {
	t.Parallel()
	rec := serveGet(t, &fakeAppStore{found: false}, "/v1/applications/471/missing")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("404 must be bodyless, got %s", rec.Body)
	}
}

func TestHandler_GetByAuthorityAndName_StoreError(t *testing.T) {
	t.Parallel()
	rec := serveGet(t, &fakeAppStore{err: context.DeadlineExceeded}, "/v1/applications/471/x")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
}

func TestHandler_GetByAuthorityAndName_RefreshesOnTap(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	refresher := &fakeRefresher{}
	rec := serveGetWith(t, &fakeAppStore{app: a, found: true}, refresher, testResolver(), "auth0|u", "/v1/applications/471/24/0123/FUL")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if len(refresher.calls) != 1 {
		t.Fatalf("expected one refresh call, got %d", len(refresher.calls))
	}
	if refresher.calls[0].userID != "auth0|u" || refresher.calls[0].app.UID != a.UID {
		t.Errorf("refresh call: %+v", refresher.calls[0])
	}
}

func TestHandler_GetByAuthorityAndName_RefreshFailureDoesNotFailRead(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	refresher := &fakeRefresher{err: context.DeadlineExceeded}
	rec := serveGetWith(t, &fakeAppStore{app: a, found: true}, refresher, testResolver(), "auth0|u", "/v1/applications/471/24/0123/FUL")

	if rec.Code != http.StatusOK {
		t.Fatalf("refresh error must not fail the read: got %d, want 200", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("body must still be written on refresh failure")
	}
}

func TestHandler_GetByAuthorityAndName_NoRefreshWhenNotFound(t *testing.T) {
	t.Parallel()
	refresher := &fakeRefresher{}
	rec := serveGetWith(t, &fakeAppStore{found: false}, refresher, testResolver(), "auth0|u", "/v1/applications/471/missing")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rec.Code)
	}
	if len(refresher.calls) != 0 {
		t.Errorf("must not refresh a missing application: %+v", refresher.calls)
	}
}

func TestHandler_GetByAuthorityAndName_NoRefreshWhenAnonymous(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	refresher := &fakeRefresher{}
	rec := serveGetWith(t, &fakeAppStore{app: a, found: true}, refresher, testResolver(), "", "/v1/applications/471/24/0123/FUL")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if len(refresher.calls) != 0 {
		t.Errorf("must not refresh without an authenticated subject: %+v", refresher.calls)
	}
}

// TestHandler_GetByAuthorityAndName_AuthoritySlugFallsBackToSlugifyAreaName pins
// the fallback branch: when the resolver doesn't know the AreaID, authoritySlug is
// authorities.Slugify(AreaName). "City of London" -> "city-of-london".
func TestHandler_GetByAuthorityAndName_AuthoritySlugFallsBackToSlugifyAreaName(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	resolver := &fakeResolver{slugToID: map[string]int{}, idToSlug: map[int]string{}}

	rec := serveGetWith(t, &fakeAppStore{app: a, found: true}, nil, resolver, "auth0|u", "/v1/applications/471/24/0123/FUL")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["authoritySlug"] != "city-of-london" {
		t.Errorf("authoritySlug fallback: got %v, want city-of-london", got["authoritySlug"])
	}
}

// ── GET /v1/applications/by-slug/{authoritySlug}/{ref...} (anonymous) ──────────

func TestHandler_GetBySlug_Found(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	store := &fakeAppStore{app: a, found: true}

	// Anonymous (no subject): resolve slug -> area id 471 -> stringified authority
	// code, then point-read the app with the slash-bearing ref captured whole.
	rec := serveGetWith(t, store, nil, testResolver(), "", "/v1/applications/by-slug/city-of-london/24/0123/FUL")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if store.lastAuthorityCode != "471" || store.lastName != "24/0123/FUL" {
		t.Errorf("routing: authorityCode=%q name=%q", store.lastAuthorityCode, store.lastName)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["uid"] != a.UID {
		t.Errorf("uid: got %v", got["uid"])
	}
	if got["authoritySlug"] != "city-of-london" {
		t.Errorf("authoritySlug: got %v, want city-of-london", got["authoritySlug"])
	}
}

// TestHandler_GetBySlug_SameBodyAsById proves the by-slug read returns byte-for-byte
// the same body as the authed by-id read (both carry the additive authoritySlug).
func TestHandler_GetBySlug_SameBodyAsById(t *testing.T) {
	t.Parallel()
	a := testApplication(t)

	byID := serveGetWith(t, &fakeAppStore{app: a, found: true}, nil, testResolver(), "", "/v1/applications/471/24/0123/FUL")
	bySlug := serveGetWith(t, &fakeAppStore{app: a, found: true}, nil, testResolver(), "", "/v1/applications/by-slug/city-of-london/24/0123/FUL")

	if byID.Code != http.StatusOK || bySlug.Code != http.StatusOK {
		t.Fatalf("codes: byId=%d bySlug=%d", byID.Code, bySlug.Code)
	}
	if byID.Body.String() != bySlug.Body.String() {
		t.Errorf("bodies differ:\n by-id:   %s\n by-slug: %s", byID.Body.String(), bySlug.Body.String())
	}
}

func TestHandler_GetBySlug_UnknownSlug(t *testing.T) {
	t.Parallel()
	store := &fakeAppStore{app: testApplication(t), found: true}
	rec := serveGetWith(t, store, nil, testResolver(), "", "/v1/applications/by-slug/no-such-place/24/0123/FUL")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("404 must be bodyless, got %s", rec.Body)
	}
	// An unknown slug short-circuits before the store read.
	if store.lastAuthorityCode != "" {
		t.Errorf("store must not be queried on unknown slug, got %q", store.lastAuthorityCode)
	}
}

func TestHandler_GetBySlug_UnknownRef(t *testing.T) {
	t.Parallel()
	rec := serveGetWith(t, &fakeAppStore{found: false}, nil, testResolver(), "", "/v1/applications/by-slug/city-of-london/missing")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("404 must be bodyless, got %s", rec.Body)
	}
}

// TestHandler_GetBySlug_DoesNotRefreshOnTap pins the anonymity of the by-slug
// route: even with an authenticated subject on the context and a refresher wired,
// it must never trigger refresh-on-tap (it reads no user data).
func TestHandler_GetBySlug_DoesNotRefreshOnTap(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	refresher := &fakeRefresher{}

	rec := serveGetWith(t, &fakeAppStore{app: a, found: true}, refresher, testResolver(), "auth0|u", "/v1/applications/by-slug/city-of-london/24/0123/FUL")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if len(refresher.calls) != 0 {
		t.Errorf("by-slug must not refresh-on-tap: %+v", refresher.calls)
	}
}

// TestHandler_Routes_BySlugAndByIdCoexist proves Go 1.22's ServeMux accepts both
// patterns without a conflict panic (the literal "by-slug" segment is more
// specific than {authorityCode}) and routes each to its own handler.
func TestHandler_Routes_BySlugAndByIdCoexist(t *testing.T) {
	t.Parallel()
	a := testApplication(t)

	byIDStore := &fakeAppStore{app: a, found: true}
	recByID := serveGetWith(t, byIDStore, nil, testResolver(), "", "/v1/applications/999/x/y/z")
	if recByID.Code != http.StatusOK {
		t.Fatalf("by-id status: got %d, want 200", recByID.Code)
	}
	if byIDStore.lastAuthorityCode != "999" || byIDStore.lastName != "x/y/z" {
		t.Errorf("by-id routed wrong: code=%q name=%q", byIDStore.lastAuthorityCode, byIDStore.lastName)
	}

	bySlugStore := &fakeAppStore{app: a, found: true}
	recBySlug := serveGetWith(t, bySlugStore, nil, testResolver(), "", "/v1/applications/by-slug/city-of-london/x/y/z")
	if recBySlug.Code != http.StatusOK {
		t.Fatalf("by-slug status: got %d, want 200", recBySlug.Code)
	}
	if bySlugStore.lastAuthorityCode != "471" || bySlugStore.lastName != "x/y/z" {
		t.Errorf("by-slug routed wrong: code=%q name=%q", bySlugStore.lastAuthorityCode, bySlugStore.lastName)
	}
}
