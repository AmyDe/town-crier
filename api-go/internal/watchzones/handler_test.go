package watchzones

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

const testUser = "auth0|user"

// capturingHandler is a hand-written slog.Handler that records emitted records so
// a test can assert a specific log line (and its attributes/level) was, or was
// not, produced.
type capturingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *capturingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}

func (h *capturingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *capturingHandler) WithGroup(string) slog.Handler      { return h }

// find returns the attributes of the first record matching level and message.
func (h *capturingHandler) find(level slog.Level, msg string) (map[string]any, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, r := range h.records {
		if r.Level == level && r.Message == msg {
			attrs := map[string]any{}
			r.Attrs(func(a slog.Attr) bool {
				attrs[a.Key] = a.Value.Any()
				return true
			})
			return attrs, true
		}
	}
	return nil, false
}

// fakeZoneStore is a hand-written zoneStore. It holds zones in an ordered slice
// so list order is deterministic, and exposes hooks for forced errors.
type fakeZoneStore struct {
	zones     []WatchZone
	getErr    error
	listErr   error
	saveErr   error
	deleteErr error
	saved     *WatchZone
	deleted   []string
}

func (f *fakeZoneStore) GetByUserID(_ context.Context, _ string) ([]WatchZone, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.zones, nil
}

func (f *fakeZoneStore) Get(_ context.Context, _, zoneID string) (WatchZone, error) {
	if f.getErr != nil {
		return WatchZone{}, f.getErr
	}
	for _, z := range f.zones {
		if z.ID == zoneID {
			return z, nil
		}
	}
	return WatchZone{}, ErrNotFound
}

func (f *fakeZoneStore) Save(_ context.Context, z WatchZone) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	saved := z
	f.saved = &saved
	return nil
}

func (f *fakeZoneStore) Delete(_ context.Context, _, zoneID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	for _, z := range f.zones {
		if z.ID == zoneID {
			f.deleted = append(f.deleted, zoneID)
			return nil
		}
	}
	return ErrNotFound
}

func testMux(t *testing.T, store zoneStore) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	Routes(mux, store, slog.New(slog.DiscardHandler))
	return mux
}

func doReq(t *testing.T, mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req := httptest.NewRequestWithContext(auth.WithSubject(ctx, testUser), method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestHandler_List(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	store := &fakeZoneStore{zones: []WatchZone{z}}
	rec := doReq(t, testMux(t, store), http.MethodGet, "/v1/me/watch-zones", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type: got %q", ct)
	}
	var got struct {
		Zones []watchZoneSummary `json:"zones"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Zones) != 1 || got.Zones[0].ID != z.ID || got.Zones[0].AuthorityID != 0 {
		t.Errorf("zones: got %+v", got.Zones)
	}
}

func TestHandler_List_EmptyArray(t *testing.T) {
	t.Parallel()
	rec := doReq(t, testMux(t, &fakeZoneStore{}), http.MethodGet, "/v1/me/watch-zones", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"zones":[]}` {
		t.Errorf("empty list body: got %s, want {\"zones\":[]}", body)
	}
}

func TestHandler_Patch_UpdatesAndReturnsZone(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	store := &fakeZoneStore{zones: []WatchZone{z}}
	body := `{"name":"Office","radiusMetres":2500,"pushEnabled":false}`
	rec := doReq(t, testMux(t, store), http.MethodPatch, "/v1/me/watch-zones/"+z.ID, body)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	var got struct {
		Zone watchZoneSummary `json:"zone"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Zone.Name != "Office" || got.Zone.RadiusMetres != 2500 || got.Zone.PushEnabled {
		t.Errorf("zone not updated: %+v", got.Zone)
	}
	if store.saved == nil || store.saved.Name != "Office" {
		t.Errorf("zone not persisted: %+v", store.saved)
	}
	// Unset fields preserved through the merge.
	if got.Zone.Latitude != z.Latitude {
		t.Errorf("unset fields changed: %+v", got.Zone)
	}
	// Back-compat shim (tc-9nbs4.6): authorityId must be present on the wire,
	// not just zero-valued after decode (absence also decodes as 0).
	var raw struct {
		Zone map[string]json.RawMessage `json:"zone"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw: %v", err)
	}
	authorityRaw, ok := raw.Zone["authorityId"]
	var authorityID int
	if !ok || json.Unmarshal(authorityRaw, &authorityID) != nil || authorityID != 0 {
		t.Errorf("authorityId: got %s, want 0", authorityRaw)
	}
}

func TestHandler_Patch_UpdatesEveryMappedField(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	store := &fakeZoneStore{zones: []WatchZone{z}}
	body := `{
		"name":"Renamed Office",
		"latitude":52.4862,
		"longitude":-1.8904,
		"radiusMetres":3000,
		"pushEnabled":false,
		"emailInstantEnabled":false
	}`
	rec := doReq(t, testMux(t, store), http.MethodPatch, "/v1/me/watch-zones/"+z.ID, body)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	var got struct {
		Zone watchZoneSummary `json:"zone"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := watchZoneSummary{
		ID:           z.ID,
		Name:         "Renamed Office",
		Latitude:     52.4862,
		Longitude:    -1.8904,
		RadiusMetres: 3000,
		// Back-compat shim (tc-9nbs4.6): always 0, never resolved.
		AuthorityID:         0,
		PushEnabled:         false,
		EmailInstantEnabled: false,
		Paused:              got.Zone.Paused,
	}
	if got.Zone != want {
		t.Errorf("response zone: got %+v, want %+v", got.Zone, want)
	}
	// Back-compat shim (tc-9nbs4.6): authorityId must be present on the wire,
	// not just zero-valued after decode (absence also decodes as 0).
	var raw struct {
		Zone map[string]json.RawMessage `json:"zone"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw: %v", err)
	}
	authorityRaw, ok := raw.Zone["authorityId"]
	var authorityID int
	if !ok || json.Unmarshal(authorityRaw, &authorityID) != nil || authorityID != 0 {
		t.Errorf("authorityId: got %s, want 0", authorityRaw)
	}
	if store.saved == nil {
		t.Fatal("zone not persisted")
	}
	if store.saved.Name != "Renamed Office" || store.saved.Latitude != 52.4862 ||
		store.saved.Longitude != -1.8904 || store.saved.RadiusMetres != 3000 ||
		store.saved.PushEnabled || store.saved.EmailInstantEnabled {
		t.Errorf("persisted zone: got %+v", store.saved)
	}
}

func TestHandler_Patch_RangeInvalid(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	tests := []struct {
		name string
		body string
	}{
		{"latitude too high", `{"latitude":91}`},
		{"longitude too low", `{"longitude":-181}`},
		{"zero radius", `{"radiusMetres":0}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := &fakeZoneStore{zones: []WatchZone{z}}
			rec := doReq(t, testMux(t, store), http.MethodPatch, "/v1/me/watch-zones/"+z.ID, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d, want 400", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
				t.Errorf("content-type: got %q", ct)
			}
			var got map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode error body: %v", err)
			}
			if got["error"] != "Invalid watch zone payload." {
				t.Errorf("error field: got %v", got["error"])
			}
			if v, ok := got["message"]; !ok || v != nil {
				t.Errorf("message must be present and null: got %v (present=%v)", v, ok)
			}
			if store.saved != nil {
				t.Error("invalid patch must not persist")
			}
		})
	}
}

func TestHandler_Patch_NotFound(t *testing.T) {
	t.Parallel()
	rec := doReq(t, testMux(t, &fakeZoneStore{}), http.MethodPatch, "/v1/me/watch-zones/missing", `{"name":"X"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("404 must be bodyless, got %s", rec.Body)
	}
}

func TestHandler_Patch_BlankNameIsServerError(t *testing.T) {
	t.Parallel()
	// Name is not validated at the endpoint; WithUpdates' guard rejects a blank
	// name, surfacing as a 500.
	z := testZone(t)
	store := &fakeZoneStore{zones: []WatchZone{z}}
	rec := doReq(t, testMux(t, store), http.MethodPatch, "/v1/me/watch-zones/"+z.ID, `{"name":"   "}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
}

// TestHandler_Patch_DuplicateNameIs409 proves that a Store.Save failure
// carrying ErrDuplicateName (a unique-violation on watch_zones' UNIQUE
// (user_id, name) constraint against a different, already-existing row) maps
// to 409 zone_name_taken rather than falling through to the generic 500
// (GH#1083, tc-h4y98).
func TestHandler_Patch_DuplicateNameIs409(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	store := &fakeZoneStore{zones: []WatchZone{z}, saveErr: ErrDuplicateName}
	rec := doReq(t, testMux(t, store), http.MethodPatch, "/v1/me/watch-zones/"+z.ID, `{"name":"Taken Name"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want 409 (body %s)", rec.Code, rec.Body)
	}
	var env apiErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if env.Error != zoneNameTakenCode {
		t.Errorf("error code: got %q, want %q", env.Error, zoneNameTakenCode)
	}
}

// --- PATCH boundary tri-state tests (tc-6he3x.4) ----------------------------

// TestHandler_Patch_Boundary_AbsentLeavesShapeUnchanged proves an absent
// "boundary" key leaves an existing custom shape untouched (acceptance
// criteria 3, the "absent" tri-state branch), and does not require a profile
// reader to be wired.
func TestHandler_Patch_Boundary_AbsentLeavesShapeUnchanged(t *testing.T) {
	t.Parallel()
	base := testZone(t)
	z, err := base.WithBoundary(squareVertices(base.Latitude, base.Longitude, 500))
	if err != nil {
		t.Fatalf("WithBoundary: %v", err)
	}
	store := &fakeZoneStore{zones: []WatchZone{z}}
	rec := doReq(t, testMux(t, store), http.MethodPatch, "/v1/me/watch-zones/"+z.ID, `{"name":"Renamed"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if store.saved == nil || !store.saved.IsCustomShape() {
		t.Fatalf("boundary must be untouched by an absent field: %+v", store.saved)
	}
	if !reflect.DeepEqual(store.saved.Boundary, z.Boundary) {
		t.Errorf("boundary changed: got %+v, want %+v", store.saved.Boundary, z.Boundary)
	}
	var got struct {
		Zone watchZoneSummary `json:"zone"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Zone.Boundary == nil {
		t.Error("response boundary must remain non-null")
	}
}

// TestHandler_Patch_Boundary_ExplicitNullConvertsToCircle proves a literal
// `"boundary":null` reverts a custom-shape zone to a circle while preserving
// its existing centre and radius (acceptance criteria 3), and that this
// revert path needs no tier check at all -- testMux wires no profile reader,
// and the request still succeeds, because the tier gate only applies to
// setting a non-null boundary (acceptance criteria 4).
func TestHandler_Patch_Boundary_ExplicitNullConvertsToCircle(t *testing.T) {
	t.Parallel()
	base := testZone(t)
	z, err := base.WithBoundary(squareVertices(base.Latitude, base.Longitude, 500))
	if err != nil {
		t.Fatalf("WithBoundary: %v", err)
	}
	wantLat, wantLon, wantRadius := z.Latitude, z.Longitude, z.RadiusMetres
	store := &fakeZoneStore{zones: []WatchZone{z}}
	rec := doReq(t, testMux(t, store), http.MethodPatch, "/v1/me/watch-zones/"+z.ID, `{"boundary":null}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if store.saved == nil || store.saved.IsCustomShape() {
		t.Fatalf("zone must revert to a circle: %+v", store.saved)
	}
	if store.saved.Latitude != wantLat || store.saved.Longitude != wantLon || store.saved.RadiusMetres != wantRadius {
		t.Errorf("centre/radius must be preserved on revert: got (%v,%v,%v), want (%v,%v,%v)",
			store.saved.Latitude, store.saved.Longitude, store.saved.RadiusMetres, wantLat, wantLon, wantRadius)
	}
	var got struct {
		Zone watchZoneSummary `json:"zone"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Zone.Boundary != nil {
		t.Error("response boundary must be null after reverting to a circle")
	}
}

// TestHandler_Patch_Boundary_ValueRequiresPaidTier403 proves a Free-tier
// caller's PATCH that sets a non-null boundary is rejected with 403
// boundary_requires_paid_tier (acceptance criteria 4) and never persisted.
func TestHandler_Patch_Boundary_ValueRequiresPaidTier403(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	store := &fakeZoneStore{zones: []WatchZone{z}}
	mux := http.NewServeMux()
	Routes(mux, store, slog.New(slog.DiscardHandler), WithProfileReader(&fakeProfileReader{profile: freeProfile(t)}))

	body := mustJSON(t, struct {
		Boundary *boundaryGeoJSON `json:"boundary"`
	}{Boundary: boundaryToGeoJSON(mustBoundary(t, squareVertices(z.Latitude, z.Longitude, 500)))})
	rec := doReq(t, mux, http.MethodPatch, "/v1/me/watch-zones/"+z.ID, body)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want 403 (body %s)", rec.Code, rec.Body)
	}
	var env apiErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if env.Error != boundaryRequiresPaidTierCode {
		t.Errorf("error code: got %q, want %q", env.Error, boundaryRequiresPaidTierCode)
	}
	if store.saved != nil {
		t.Error("must not save a zone when the boundary tier gate rejects the request")
	}
}

// TestHandler_Patch_Boundary_ValueSetsShape proves a paid-tier caller's PATCH
// that sets a valid boundary derives and persists the new centroid/enclosing
// radius (acceptance criteria 3's "value" branch).
func TestHandler_Patch_Boundary_ValueSetsShape(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	b := mustBoundary(t, squareVertices(z.Latitude, z.Longitude, 500))
	wantLat, wantLon := b.Centroid()
	wantRadius := b.EnclosingRadiusMetres()

	store := &fakeZoneStore{zones: []WatchZone{z}}
	mux := http.NewServeMux()
	Routes(mux, store, slog.New(slog.DiscardHandler), WithProfileReader(&fakeProfileReader{profile: personalProfile(t)}))

	body := mustJSON(t, struct {
		Boundary *boundaryGeoJSON `json:"boundary"`
	}{Boundary: boundaryToGeoJSON(b)})
	rec := doReq(t, mux, http.MethodPatch, "/v1/me/watch-zones/"+z.ID, body)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if store.saved == nil || !store.saved.IsCustomShape() {
		t.Fatalf("zone must become a custom shape: %+v", store.saved)
	}
	if store.saved.Latitude != wantLat || store.saved.Longitude != wantLon || store.saved.RadiusMetres != wantRadius {
		t.Errorf("derived fields: got (%v,%v,%v), want (%v,%v,%v)",
			store.saved.Latitude, store.saved.Longitude, store.saved.RadiusMetres, wantLat, wantLon, wantRadius)
	}
}

// TestHandler_Patch_Boundary_TooLargeIs400 proves a paid-tier caller's PATCH
// that sets a boundary exceeding their own tier's radius cap (but under the
// hard server ceiling) is rejected with 400 boundary_too_large and never
// persisted (acceptance criteria 6, Gate B).
func TestHandler_Patch_Boundary_TooLargeIs400(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	// Personal tier caps at 5000m; a 4000m half-extent square's corner-derived
	// enclosing radius (~5657m) exceeds it while staying under the hard 10000m
	// ceiling, isolating Gate B (the tier cap) from Gate A (the hard ceiling).
	b := mustBoundary(t, squareVertices(z.Latitude, z.Longitude, 4000))
	store := &fakeZoneStore{zones: []WatchZone{z}}
	mux := http.NewServeMux()
	Routes(mux, store, slog.New(slog.DiscardHandler), WithProfileReader(&fakeProfileReader{profile: personalProfile(t)}))

	body := mustJSON(t, struct {
		Boundary *boundaryGeoJSON `json:"boundary"`
	}{Boundary: boundaryToGeoJSON(b)})
	rec := doReq(t, mux, http.MethodPatch, "/v1/me/watch-zones/"+z.ID, body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400 (body %s)", rec.Code, rec.Body)
	}
	var env apiErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if env.Error != boundaryTooLargeCode {
		t.Errorf("error code: got %q, want %q", env.Error, boundaryTooLargeCode)
	}
	if store.saved != nil {
		t.Error("must not save a zone whose boundary is too large")
	}
}

// TestHandler_Patch_Boundary_InvalidShapeIs400 proves a paid-tier caller's
// PATCH that sets a malformed shape (here, fewer than the domain's minimum 3
// vertices) surfaces as 400 boundary_invalid via WithUpdates' own NewBoundary
// validation, not a re-implemented HTTP-layer check (acceptance criteria 5).
func TestHandler_Patch_Boundary_InvalidShapeIs400(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	store := &fakeZoneStore{zones: []WatchZone{z}}
	mux := http.NewServeMux()
	Routes(mux, store, slog.New(slog.DiscardHandler), WithProfileReader(&fakeProfileReader{profile: proProfile(t)}))

	body := `{"boundary":{"type":"Polygon","coordinates":[[[-0.12,51.5],[-0.11,51.51]]]}}`
	rec := doReq(t, mux, http.MethodPatch, "/v1/me/watch-zones/"+z.ID, body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400 (body %s)", rec.Code, rec.Body)
	}
	var env apiErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if env.Error != boundaryInvalidCode {
		t.Errorf("error code: got %q, want %q", env.Error, boundaryInvalidCode)
	}
	if store.saved != nil {
		t.Error("must not save a zone with an invalid boundary")
	}
}

// TestHandler_Patch_Boundary_NoProfileReaderWiredIs500 proves that attempting
// to SET a boundary without a profile reader wired is a 500 (a wiring bug,
// never reachable in production), not a silent allow or a false 403 -- the
// tier entitlement genuinely cannot be checked.
func TestHandler_Patch_Boundary_NoProfileReaderWiredIs500(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	store := &fakeZoneStore{zones: []WatchZone{z}}
	body := mustJSON(t, struct {
		Boundary *boundaryGeoJSON `json:"boundary"`
	}{Boundary: boundaryToGeoJSON(mustBoundary(t, squareVertices(z.Latitude, z.Longitude, 500)))})
	rec := doReq(t, testMux(t, store), http.MethodPatch, "/v1/me/watch-zones/"+z.ID, body)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
	if store.saved != nil {
		t.Error("must not save when the tier gate cannot be verified")
	}
}

// TestHandler_Patch_Boundary_ResendingUnchangedValueSucceedsForDowngradedTier
// proves the fix for tc-h3aov (the same defect pattern tc-k3ncu fixed for the
// filter gate): a downgraded (Free-tier) caller who already owns a
// custom-shape zone and resends its UNCHANGED boundary vertices alongside an
// unrelated field edit (name here — the same pattern a client that always
// PATCHes full state on every edit produces) must succeed, because nothing
// about the boundary actually changed. Before the fix, settingBoundary gated
// on mere non-emptiness of the incoming boundary and always 403'd here.
func TestHandler_Patch_Boundary_ResendingUnchangedValueSucceedsForDowngradedTier(t *testing.T) {
	t.Parallel()
	base := testZone(t)
	z, err := base.WithBoundary(squareVertices(base.Latitude, base.Longitude, 500))
	if err != nil {
		t.Fatalf("WithBoundary: %v", err)
	}
	store := &fakeZoneStore{zones: []WatchZone{z}}
	mux := http.NewServeMux()
	Routes(mux, store, slog.New(slog.DiscardHandler), WithProfileReader(&fakeProfileReader{profile: freeProfile(t)}))

	body := mustJSON(t, struct {
		Name     *string          `json:"name"`
		Boundary *boundaryGeoJSON `json:"boundary"`
	}{Name: strPtr("Renamed"), Boundary: boundaryToGeoJSON(z.Boundary)})
	rec := doReq(t, mux, http.MethodPatch, "/v1/me/watch-zones/"+z.ID, body)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if store.saved == nil {
		t.Fatal("zone must be saved")
	}
	if store.saved.Name != "Renamed" {
		t.Errorf("name: got %q, want %q", store.saved.Name, "Renamed")
	}
	if !store.saved.Boundary.Equal(z.Boundary) {
		t.Errorf("boundary must be unchanged: got %+v, want %+v", store.saved.Boundary, z.Boundary)
	}
}

// TestHandler_Patch_Boundary_ChangingExistingBoundaryStillRequiresPaidTier403
// proves the unchanged-resend carve-out (above) does not weaken the tier gate
// itself: a downgraded (Free-tier) caller who owns a custom-shape zone and
// sends a genuinely DIFFERENT set of boundary vertices still gets 403
// boundary_requires_paid_tier, and the update is never persisted. Complements
// TestHandler_Patch_Boundary_ValueRequiresPaidTier403, which only covers a
// zone with no pre-existing boundary (a genuine new-set case) -- this one
// proves the gate still fires when the zone already has a (different) shape.
func TestHandler_Patch_Boundary_ChangingExistingBoundaryStillRequiresPaidTier403(t *testing.T) {
	t.Parallel()
	base := testZone(t)
	z, err := base.WithBoundary(squareVertices(base.Latitude, base.Longitude, 500))
	if err != nil {
		t.Fatalf("WithBoundary: %v", err)
	}
	store := &fakeZoneStore{zones: []WatchZone{z}}
	mux := http.NewServeMux()
	Routes(mux, store, slog.New(slog.DiscardHandler), WithProfileReader(&fakeProfileReader{profile: freeProfile(t)}))

	body := mustJSON(t, struct {
		Boundary *boundaryGeoJSON `json:"boundary"`
	}{Boundary: boundaryToGeoJSON(mustBoundary(t, squareVertices(z.Latitude, z.Longitude, 700)))})
	rec := doReq(t, mux, http.MethodPatch, "/v1/me/watch-zones/"+z.ID, body)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want 403 (body %s)", rec.Code, rec.Body)
	}
	var env apiErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if env.Error != boundaryRequiresPaidTierCode {
		t.Errorf("error code: got %q, want %q", env.Error, boundaryRequiresPaidTierCode)
	}
	if store.saved != nil {
		t.Error("must not save a zone when the boundary tier gate rejects the request")
	}
}

// TestHandler_Patch_NotFound_TakesPrecedenceOverBoundaryTierGate proves a
// PATCH to a non-existent (or not-owned) zone ID returns 404, not a
// boundary-tier 403, even when the request body sets a boundary and the
// caller's tier would otherwise fail the gate -- the reviewer-flagged
// precedence question for tc-k3ncu's identical filter-gate fix, verified here
// for the boundary gate (main does not have a merged fix for either as of
// tc-h3aov).
func TestHandler_Patch_NotFound_TakesPrecedenceOverBoundaryTierGate(t *testing.T) {
	t.Parallel()
	store := &fakeZoneStore{}
	mux := http.NewServeMux()
	Routes(mux, store, slog.New(slog.DiscardHandler), WithProfileReader(&fakeProfileReader{profile: freeProfile(t)}))

	body := mustJSON(t, struct {
		Boundary *boundaryGeoJSON `json:"boundary"`
	}{Boundary: boundaryToGeoJSON(mustBoundary(t, squareVertices(51.5074, -0.1278, 500)))})
	rec := doReq(t, mux, http.MethodPatch, "/v1/me/watch-zones/missing", body)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404 (body %s)", rec.Code, rec.Body)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("404 must be bodyless, got %s", rec.Body)
	}
	if store.saved != nil {
		t.Error("must not save when the zone does not exist")
	}
}

// --- PATCH filterKey tri-state tests (GH#1090, epic tc-w825j) --------------

// TestHandler_Patch_FilterKey_FreeOrPersonalTierIs403 proves a non-Pro
// caller's PATCH that SETS a non-empty filterKey is rejected with 403
// filter_requires_pro_tier and never persisted.
func TestHandler_Patch_FilterKey_FreeOrPersonalTierIs403(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		profile func(*testing.T) *profiles.UserProfile
	}{
		{"Free tier", freeProfile},
		{"Personal tier", personalProfile},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			z := testZone(t)
			store := &fakeZoneStore{zones: []WatchZone{z}}
			mux := http.NewServeMux()
			Routes(mux, store, slog.New(slog.DiscardHandler), WithProfileReader(&fakeProfileReader{profile: tc.profile(t)}))

			body := mustJSON(t, struct {
				FilterKey *string `json:"filterKey"`
			}{FilterKey: strPtr(string(FilterKeyHouseBuilder))})
			rec := doReq(t, mux, http.MethodPatch, "/v1/me/watch-zones/"+z.ID, body)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status: got %d, want 403 (body %s)", rec.Code, rec.Body)
			}
			var env apiErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
				t.Fatalf("decode error envelope: %v", err)
			}
			if env.Error != filterRequiresProTierCode {
				t.Errorf("error code: got %q, want %q", env.Error, filterRequiresProTierCode)
			}
			if store.saved != nil {
				t.Error("must not save when the filter tier gate rejects the request")
			}
		})
	}
}

// TestHandler_Patch_FilterKey_ProTierSetsAndRoundTrips proves a Pro-tier
// caller's PATCH that sets a valid filterKey succeeds (200), persists it, and
// round-trips it in the response.
func TestHandler_Patch_FilterKey_ProTierSetsAndRoundTrips(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	store := &fakeZoneStore{zones: []WatchZone{z}}
	mux := http.NewServeMux()
	Routes(mux, store, slog.New(slog.DiscardHandler), WithProfileReader(&fakeProfileReader{profile: proProfile(t)}))

	body := mustJSON(t, struct {
		FilterKey *string `json:"filterKey"`
	}{FilterKey: strPtr(string(FilterKeyHMOHouseShares))})
	rec := doReq(t, mux, http.MethodPatch, "/v1/me/watch-zones/"+z.ID, body)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if store.saved == nil || store.saved.FilterKey != FilterKeyHMOHouseShares {
		t.Fatalf("saved zone filter key: got %+v", store.saved)
	}
	var got struct {
		Zone watchZoneSummary `json:"zone"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Zone.FilterKey == nil || *got.Zone.FilterKey != string(FilterKeyHMOHouseShares) {
		t.Errorf("response filterKey: got %v, want %q", got.Zone.FilterKey, FilterKeyHMOHouseShares)
	}
}

// TestHandler_Patch_FilterKey_ExplicitNullClearsRegardlessOfTier proves a
// literal `"filterKey": null` always clears an existing filter back to
// unfiltered, with no tier check at all -- the test wires no profile reader,
// and the request still succeeds, mirroring the boundary revert's no-gate
// behaviour.
func TestHandler_Patch_FilterKey_ExplicitNullClearsRegardlessOfTier(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	z.FilterKey = FilterKeyHouseBuilder
	store := &fakeZoneStore{zones: []WatchZone{z}}
	rec := doReq(t, testMux(t, store), http.MethodPatch, "/v1/me/watch-zones/"+z.ID, `{"filterKey":null}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if store.saved == nil || store.saved.FilterKey != "" {
		t.Fatalf("filter key must be cleared: got %+v", store.saved)
	}
	var got struct {
		Zone watchZoneSummary `json:"zone"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Zone.FilterKey != nil {
		t.Error("response filterKey must be null after clearing")
	}
}

// TestHandler_Patch_FilterKey_AbsentLeavesExistingFilterUntouched proves
// omitting the "filterKey" key entirely leaves an existing filter alone, and
// requires no profile reader (mirrors the boundary absent-key test).
func TestHandler_Patch_FilterKey_AbsentLeavesExistingFilterUntouched(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	z.FilterKey = FilterKeyNewHomes
	store := &fakeZoneStore{zones: []WatchZone{z}}
	rec := doReq(t, testMux(t, store), http.MethodPatch, "/v1/me/watch-zones/"+z.ID, `{"name":"Renamed"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if store.saved == nil || store.saved.FilterKey != FilterKeyNewHomes {
		t.Fatalf("filter key must be untouched by an absent field: got %+v", store.saved)
	}
	var got struct {
		Zone watchZoneSummary `json:"zone"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Zone.FilterKey == nil || *got.Zone.FilterKey != string(FilterKeyNewHomes) {
		t.Errorf("response filterKey: got %v, want %q", got.Zone.FilterKey, FilterKeyNewHomes)
	}
}

// TestHandler_Patch_FilterKey_UnrecognisedIs400 proves an unrecognised
// filterKey string maps to 400 filter_key_invalid (via WithUpdates'
// ErrUnknownFilterKey), even for a Pro-tier caller who has passed the
// entitlement gate.
func TestHandler_Patch_FilterKey_UnrecognisedIs400(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	store := &fakeZoneStore{zones: []WatchZone{z}}
	mux := http.NewServeMux()
	Routes(mux, store, slog.New(slog.DiscardHandler), WithProfileReader(&fakeProfileReader{profile: proProfile(t)}))

	body := mustJSON(t, struct {
		FilterKey *string `json:"filterKey"`
	}{FilterKey: strPtr("not_a_real_filter")})
	rec := doReq(t, mux, http.MethodPatch, "/v1/me/watch-zones/"+z.ID, body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400 (body %s)", rec.Code, rec.Body)
	}
	var env apiErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if env.Error != filterKeyInvalidCode {
		t.Errorf("error code: got %q, want %q", env.Error, filterKeyInvalidCode)
	}
	if store.saved != nil {
		t.Error("must not save a zone with an unrecognised filter key")
	}
}

// TestHandler_Patch_FilterKey_NoProfileReaderWiredIs500 proves that attempting
// to SET a filterKey without a profile reader wired is a 500 (a wiring bug),
// mirroring the boundary tier gate's equivalent test.
func TestHandler_Patch_FilterKey_NoProfileReaderWiredIs500(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	store := &fakeZoneStore{zones: []WatchZone{z}}
	body := mustJSON(t, struct {
		FilterKey *string `json:"filterKey"`
	}{FilterKey: strPtr(string(FilterKeyHouseBuilder))})
	rec := doReq(t, testMux(t, store), http.MethodPatch, "/v1/me/watch-zones/"+z.ID, body)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
	if store.saved != nil {
		t.Error("must not save when the filter tier gate cannot be verified")
	}
}

// strPtr is a tiny test helper for building an inline *string, mirroring
// mustJSON/mustBoundary's role for filterKey request bodies.
func strPtr(s string) *string { return &s }

func TestHandler_Delete(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	store := &fakeZoneStore{zones: []WatchZone{z}}
	rec := doReq(t, testMux(t, store), http.MethodDelete, "/v1/me/watch-zones/"+z.ID, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want 204", rec.Code)
	}
	if len(store.deleted) != 1 || store.deleted[0] != z.ID {
		t.Errorf("zone not deleted: %+v", store.deleted)
	}
}

func TestPatch_RejectsOversizedRadius(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		radiusMetres float64
		wantStatus   int
	}{
		{"exactly at limit is valid", 10000, http.StatusOK},
		{"just above limit is 400", 10001, http.StatusBadRequest},
		{"far above limit is 400", 1e308, http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			z := testZone(t)
			store := &fakeZoneStore{zones: []WatchZone{z}}
			body := fmt.Sprintf(`{"radiusMetres":%v}`, tc.radiusMetres)
			rec := doReq(t, testMux(t, store), http.MethodPatch, "/v1/me/watch-zones/"+z.ID, body)
			if rec.Code != tc.wantStatus {
				t.Fatalf("radiusMetres=%v: got status %d, want %d", tc.radiusMetres, rec.Code, tc.wantStatus)
			}
		})
	}
}

// threeZonesRankedByAge builds three zones for testUser with distinct,
// ascending CreatedAt timestamps so oldest-first (CreatedAt, ID) ranking is
// unambiguous: zone-1 is rank 1 (oldest), zone-2 rank 2, zone-3 rank 3.
func threeZonesRankedByAge(t *testing.T) (zone1, zone2, zone3 WatchZone) {
	t.Helper()
	mk := func(id string, day int) WatchZone {
		z, err := NewWatchZone(id, testUser, "Zone "+id, 51.5, -0.1, 1000,
			time.Date(2026, 6, day, 9, 0, 0, 0, time.UTC), true, true)
		if err != nil {
			t.Fatalf("NewWatchZone %s: %v", id, err)
		}
		return z
	}
	return mk("zone-1", 1), mk("zone-2", 2), mk("zone-3", 3)
}

func TestHandler_List_MarksPausedZonesOverEffectiveTierLimit(t *testing.T) {
	t.Parallel()
	z1, z2, z3 := threeZonesRankedByAge(t)
	store := &fakeZoneStore{zones: []WatchZone{z1, z2, z3}}
	mux := http.NewServeMux()
	// Free tier: WatchZoneLimit() == 1, so only the oldest zone stays active.
	Routes(mux, store, slog.New(slog.DiscardHandler), WithProfileReader(&fakeProfileReader{profile: freeProfile(t)}))

	rec := doReq(t, mux, http.MethodGet, "/v1/me/watch-zones", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %s)", rec.Code, rec.Body)
	}

	// Golden-check first: every pre-existing field must round-trip unchanged,
	// and "paused", "boundary" (tc-6he3x.4) and "filterKey" (GH#1090, epic
	// tc-w825j) are the only additions (11 keys total per zone).
	var raw struct {
		Zones []map[string]any `json:"zones"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw: %v", err)
	}
	if len(raw.Zones) != 3 {
		t.Fatalf("zones: got %d, want 3", len(raw.Zones))
	}
	wantKeys := []string{"id", "name", "latitude", "longitude", "radiusMetres", "authorityId", "pushEnabled", "emailInstantEnabled", "paused", "boundary", "filterKey"}
	for i, obj := range raw.Zones {
		if len(obj) != len(wantKeys) {
			t.Errorf("zone %d: got %d keys %v, want %d keys %v", i, len(obj), keysOf(obj), len(wantKeys), wantKeys)
		}
		for _, k := range wantKeys {
			if _, ok := obj[k]; !ok {
				t.Errorf("zone %d: missing pre-existing/expected key %q", i, k)
			}
		}
		// authorityId is a back-compat shim (tc-9nbs4.6): always present, always 0.
		if authorityID, ok := obj["authorityId"].(float64); !ok || authorityID != 0 {
			t.Errorf("zone %d: authorityId = %v, want 0", i, obj["authorityId"])
		}
	}

	var got struct {
		Zones []watchZoneSummary `json:"zones"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	wantPaused := map[string]bool{"zone-1": false, "zone-2": true, "zone-3": true}
	for _, z := range got.Zones {
		if z.Paused != wantPaused[z.ID] {
			t.Errorf("zone %s: paused = %v, want %v", z.ID, z.Paused, wantPaused[z.ID])
		}
		// Pre-existing fields must still be byte-identical to summaryOf's
		// direct mapping of the domain zone.
		if z.Name != "Zone "+z.ID || z.Latitude != 51.5 || z.Longitude != -0.1 ||
			z.RadiusMetres != 1000 || !z.PushEnabled || !z.EmailInstantEnabled {
			t.Errorf("zone %s: pre-existing fields changed: %+v", z.ID, z)
		}
	}
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestHandler_Patch_ReturnsPausedForEditedZone(t *testing.T) {
	t.Parallel()
	z1, z2, _ := threeZonesRankedByAge(t)
	tests := []struct {
		name       string
		zoneID     string
		wantPaused bool
	}{
		{"editing the active (rank 1) zone reports unpaused", "zone-1", false},
		{"editing an over-quota (rank 2) zone reports paused", "zone-2", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := &fakeZoneStore{zones: []WatchZone{z1, z2}}
			mux := http.NewServeMux()
			Routes(mux, store, slog.New(slog.DiscardHandler), WithProfileReader(&fakeProfileReader{profile: freeProfile(t)}))

			rec := doReq(t, mux, http.MethodPatch, "/v1/me/watch-zones/"+tc.zoneID, `{"name":"Renamed"}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("status: got %d, want 200 (body %s)", rec.Code, rec.Body)
			}
			var got struct {
				Zone watchZoneSummary `json:"zone"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got.Zone.Name != "Renamed" {
				t.Errorf("zone not updated: %+v", got.Zone)
			}
			if got.Zone.Paused != tc.wantPaused {
				t.Errorf("paused: got %v, want %v", got.Zone.Paused, tc.wantPaused)
			}
		})
	}
}

func TestHandler_List_NoProfileReaderWired_EveryZoneUnpaused(t *testing.T) {
	t.Parallel()
	// No WithProfileReader option: the pause computation must fail open rather
	// than block the list response on a missing dependency.
	z1, z2, z3 := threeZonesRankedByAge(t)
	store := &fakeZoneStore{zones: []WatchZone{z1, z2, z3}}
	rec := doReq(t, testMux(t, store), http.MethodGet, "/v1/me/watch-zones", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	var got struct {
		Zones []watchZoneSummary `json:"zones"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, z := range got.Zones {
		if z.Paused {
			t.Errorf("zone %s: paused = true with no profile reader wired, want false (fail open)", z.ID)
		}
	}
}

func TestHandler_Delete_NotFound(t *testing.T) {
	t.Parallel()
	rec := doReq(t, testMux(t, &fakeZoneStore{}), http.MethodDelete, "/v1/me/watch-zones/missing", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("404 must be bodyless, got %s", rec.Body)
	}
}
