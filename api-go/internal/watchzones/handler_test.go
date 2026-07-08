package watchzones

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
)

const testUser = "auth0|user"

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
	if len(got.Zones) != 1 || got.Zones[0].ID != z.ID || got.Zones[0].AuthorityID != z.AuthorityID {
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
	if got.Zone.AuthorityID != z.AuthorityID || got.Zone.Latitude != z.Latitude {
		t.Errorf("unset fields changed: %+v", got.Zone)
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
		{"zero authority", `{"authorityId":0}`},
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
		z, err := NewWatchZone(id, testUser, "Zone "+id, 51.5, -0.1, 1000, 99,
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
	// and "paused" must be the only addition (9 keys total per zone).
	var raw struct {
		Zones []map[string]any `json:"zones"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw: %v", err)
	}
	if len(raw.Zones) != 3 {
		t.Fatalf("zones: got %d, want 3", len(raw.Zones))
	}
	wantKeys := []string{"id", "name", "latitude", "longitude", "radiusMetres", "authorityId", "pushEnabled", "emailInstantEnabled", "paused"}
	for i, obj := range raw.Zones {
		if len(obj) != len(wantKeys) {
			t.Errorf("zone %d: got %d keys %v, want %d keys %v", i, len(obj), keysOf(obj), len(wantKeys), wantKeys)
		}
		for _, k := range wantKeys {
			if _, ok := obj[k]; !ok {
				t.Errorf("zone %d: missing pre-existing/expected key %q", i, k)
			}
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
			z.RadiusMetres != 1000 || z.AuthorityID != 99 || !z.PushEnabled || !z.EmailInstantEnabled {
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
