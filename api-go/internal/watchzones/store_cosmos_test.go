package watchzones

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// fakeItems is a hand-written stand-in for the azcosmos container item API the
// store depends on. It records each call so partition-key / id / query routing
// can be asserted, and lets a test inject stored documents or a specific error.
type fakeItems struct {
	stored       map[string][]byte // id -> raw document bytes (ReadItem source)
	readErr      error
	upsertErr    error
	deleteErr    error
	queryErr     error
	queryResult  [][]byte
	lastReadPK   string
	lastReadID   string
	lastUpsertPK string
	lastUpsert   []byte
	upserts      [][]byte // every upserted document body, in call order, for backfill assertions
	lastDeletePK string
	lastDeleteID string
	deletedIDs   []string // every (pk,id) deleted, in call order, for cascade assertions
	lastQueryPK  string
	lastQuery    string
	lastParams   map[string]any

	crossPartitionResult [][]byte // QueryItemsCrossPartition source
}

func newFakeItems() *fakeItems {
	return &fakeItems{stored: map[string][]byte{}}
}

func (f *fakeItems) ReadItem(_ context.Context, partitionKey, id string) ([]byte, error) {
	f.lastReadPK = partitionKey
	f.lastReadID = id
	if f.readErr != nil {
		return nil, f.readErr
	}
	b, ok := f.stored[id]
	if !ok {
		return nil, &azcore.ResponseError{StatusCode: http.StatusNotFound}
	}
	return b, nil
}

func (f *fakeItems) UpsertItem(_ context.Context, partitionKey string, item []byte) error {
	f.lastUpsertPK = partitionKey
	f.lastUpsert = item
	if f.upsertErr != nil {
		return f.upsertErr
	}
	// Copy the body: azcosmos owns the slice after the call, and the store reuses
	// no buffer, but copying keeps each recorded upsert independent regardless.
	saved := make([]byte, len(item))
	copy(saved, item)
	f.upserts = append(f.upserts, saved)
	return nil
}

func (f *fakeItems) DeleteItem(_ context.Context, partitionKey, id string) error {
	f.lastDeletePK = partitionKey
	f.lastDeleteID = id
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deletedIDs = append(f.deletedIDs, id)
	delete(f.stored, id)
	return nil
}

func (f *fakeItems) QueryItems(_ context.Context, partitionKey, query string, params map[string]any) ([][]byte, error) {
	f.lastQueryPK = partitionKey
	f.lastQuery = query
	f.lastParams = params
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return f.queryResult, nil
}

func (f *fakeItems) QueryItemsCrossPartition(_ context.Context, query string, params map[string]any) ([][]byte, error) {
	f.lastQuery = query
	f.lastParams = params
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return f.crossPartitionResult, nil
}

func mustDocBytes(t *testing.T, z WatchZone) []byte {
	t.Helper()
	b, err := json.Marshal(newWatchZoneDocument(z))
	if err != nil {
		t.Fatalf("marshal doc: %v", err)
	}
	return b
}

func TestCosmosStore_GetByUserID_SinglePartitionQuery(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	items := newFakeItems()
	items.queryResult = [][]byte{mustDocBytes(t, z)}
	store := NewCosmosStore(items)

	zones, err := store.GetByUserID(context.Background(), "auth0|user")
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if len(zones) != 1 || zones[0].ID != z.ID {
		t.Fatalf("zones: got %+v", zones)
	}
	if items.lastQueryPK != "auth0|user" {
		t.Errorf("query partition key: got %q, want auth0|user", items.lastQueryPK)
	}
	if items.lastParams["@userId"] != "auth0|user" {
		t.Errorf("query param @userId: got %v", items.lastParams["@userId"])
	}
}

func TestCosmosStore_GetByUserID_OrdersByZoneIDForDeterminism(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	store := NewCosmosStore(items)

	if _, err := store.GetByUserID(context.Background(), "auth0|user"); err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	// Without an ORDER BY, Cosmos returns a user's zones in an arbitrary,
	// non-deterministic order each request, which made the GDPR export's
	// zonePreferences array flake on order (tc-zgnt). The list query must order by
	// the zone id (the document id) so successive list calls are stable.
	if !strings.Contains(items.lastQuery, "ORDER BY c.id") {
		t.Errorf("list query must ORDER BY c.id for deterministic order: %q", items.lastQuery)
	}
}

func TestCosmosStore_GetByUserID_EmptyWhenNoZones(t *testing.T) {
	t.Parallel()
	store := NewCosmosStore(newFakeItems())
	zones, err := store.GetByUserID(context.Background(), "auth0|user")
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if len(zones) != 0 {
		t.Errorf("expected no zones, got %+v", zones)
	}
}

func TestCosmosStore_Get_PointReadKeyedByUserAndZone(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	items := newFakeItems()
	items.stored[z.ID] = mustDocBytes(t, z)
	store := NewCosmosStore(items)

	got, err := store.Get(context.Background(), z.UserID, z.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != z.ID {
		t.Errorf("id: got %q, want %q", got.ID, z.ID)
	}
	if items.lastReadPK != z.UserID || items.lastReadID != z.ID {
		t.Errorf("point read keys: pk=%q id=%q, want pk=%q id=%q", items.lastReadPK, items.lastReadID, z.UserID, z.ID)
	}
}

func TestCosmosStore_Get_NotFound(t *testing.T) {
	t.Parallel()
	store := NewCosmosStore(newFakeItems())
	_, err := store.Get(context.Background(), "auth0|user", "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestCosmosStore_Save_UpsertsKeyedByUser(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	items := newFakeItems()
	store := NewCosmosStore(items)

	if err := store.Save(context.Background(), z); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if items.lastUpsertPK != z.UserID {
		t.Errorf("upsert partition key: got %q, want %q", items.lastUpsertPK, z.UserID)
	}
	var doc watchZoneDocument
	if err := json.Unmarshal(items.lastUpsert, &doc); err != nil {
		t.Fatalf("unmarshal upserted doc: %v", err)
	}
	if doc.ID != z.ID || doc.UserID != z.UserID {
		t.Errorf("upserted doc identity: got %+v", doc)
	}
}

func TestCosmosStore_Delete_Success(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	items := newFakeItems()
	store := NewCosmosStore(items)

	if err := store.Delete(context.Background(), z.UserID, z.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if items.lastDeletePK != z.UserID || items.lastDeleteID != z.ID {
		t.Errorf("delete keys: pk=%q id=%q, want pk=%q id=%q", items.lastDeletePK, items.lastDeleteID, z.UserID, z.ID)
	}
}

func TestCosmosStore_Delete_NotFound(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	items.deleteErr = &azcore.ResponseError{StatusCode: http.StatusNotFound}
	store := NewCosmosStore(items)

	err := store.Delete(context.Background(), "auth0|user", "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestCosmosStore_DeleteAllByUserID_QueriesPartitionThenDeletesEachID(t *testing.T) {
	t.Parallel()
	const userID = "auth0|user"
	items := newFakeItems()
	items.queryResult = [][]byte{
		[]byte(`{"id":"zone-a"}`),
		[]byte(`{"id":"zone-b"}`),
	}
	store := NewCosmosStore(items)

	if err := store.DeleteAllByUserID(context.Background(), userID); err != nil {
		t.Fatalf("DeleteAllByUserID: %v", err)
	}

	if items.lastQueryPK != userID {
		t.Errorf("cascade query partition key: got %q, want %q", items.lastQueryPK, userID)
	}
	if len(items.deletedIDs) != 2 || items.deletedIDs[0] != "zone-a" || items.deletedIDs[1] != "zone-b" {
		t.Errorf("deleted ids: got %v, want [zone-a zone-b]", items.deletedIDs)
	}
	if items.lastDeletePK != userID {
		t.Errorf("cascade delete partition key: got %q, want %q", items.lastDeletePK, userID)
	}
}

func TestCosmosStore_DeleteAllByUserID_NoZonesIsNoOp(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	store := NewCosmosStore(items)

	if err := store.DeleteAllByUserID(context.Background(), "auth0|nobody"); err != nil {
		t.Fatalf("DeleteAllByUserID: %v", err)
	}
	if len(items.deletedIDs) != 0 {
		t.Errorf("expected no deletes, got %v", items.deletedIDs)
	}
}

func TestCosmosStore_DistinctAuthorityIDs_CrossPartition(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	// azcosmos cannot serve a cross-partition DISTINCT (the gateway 400s), so the
	// query is a plain projection (SELECT VALUE c.authorityId FROM c, bare JSON
	// numbers) and the store dedupes client-side (tc-b7cm). The fake returns the
	// same authority id more than once, out of order; the store must collapse it
	// to one each in first-seen order.
	items.crossPartitionResult = [][]byte{[]byte("10"), []byte("20"), []byte("10"), []byte("30")}
	store := NewCosmosStore(items)

	ids, err := store.DistinctAuthorityIDs(context.Background())
	if err != nil {
		t.Fatalf("DistinctAuthorityIDs: %v", err)
	}
	want := []int{10, 20, 30}
	if len(ids) != len(want) {
		t.Fatalf("ids: got %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("dedup order mismatch: got %v, want %v", ids, want)
		}
	}
	// Guard the regression: a cross-partition DISTINCT 400s at the gateway, so the
	// query the store sends must NOT contain DISTINCT.
	if strings.Contains(items.lastQuery, "DISTINCT") {
		t.Errorf("cross-partition query must not use DISTINCT (gateway 400): %q", items.lastQuery)
	}
}

func TestCosmosStore_FindZonesContaining_CrossPartitionSpatialQuery(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	items := newFakeItems()
	items.crossPartitionResult = [][]byte{mustDocBytes(t, z)}
	store := NewCosmosStore(items)

	zones, err := store.FindZonesContaining(context.Background(), z.AuthorityID, 51.5155, -0.0931)
	if err != nil {
		t.Fatalf("FindZonesContaining: %v", err)
	}
	if len(zones) != 1 || zones[0].ID != z.ID {
		t.Fatalf("zones: got %+v", zones)
	}
	// The point-in-circle predicate is an ST_DISTANCE against the zone radius,
	// run cross-partition (every user's zones).
	if !strings.Contains(items.lastQuery, "ST_DISTANCE") {
		t.Errorf("query missing ST_DISTANCE: %q", items.lastQuery)
	}
	if !strings.Contains(items.lastQuery, "c.radiusMetres") {
		t.Errorf("query must compare against c.radiusMetres: %q", items.lastQuery)
	}
	if items.lastParams["@latitude"] != 51.5155 || items.lastParams["@longitude"] != -0.0931 {
		t.Errorf("spatial params: got lat=%v lng=%v", items.lastParams["@latitude"], items.lastParams["@longitude"])
	}
}

func TestCosmosStore_FindZonesContaining_FiltersByAuthorityBeforeDistance(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	store := NewCosmosStore(items)

	// An application in authority 99 must match ONLY zones scoped to authority 99.
	// The store sends Cosmos an equality predicate on c.authorityId bound to the
	// application's authority alongside the ST_DISTANCE test, so a zone scoped to a
	// different authority is never returned even when the application falls
	// geographically within the zone's radius. This is the documented
	// authority-scoped zone model (zone.go) and mirrors the saved-bookmark path's
	// existing app.AreaID scoping. Cosmos evaluates the predicate; this test locks
	// that the predicate is present and bound to the passed authority.
	if _, err := store.FindZonesContaining(context.Background(), 99, 51.5, -0.1); err != nil {
		t.Fatalf("FindZonesContaining: %v", err)
	}
	if !strings.Contains(items.lastQuery, "c.authorityId = @authorityId") {
		t.Errorf("query must filter on c.authorityId = @authorityId: %q", items.lastQuery)
	}
	if items.lastParams["@authorityId"] != 99 {
		t.Errorf("@authorityId param: got %v, want 99", items.lastParams["@authorityId"])
	}
	// The authority equality must be ANDed with the distance test, not replace it.
	if !strings.Contains(items.lastQuery, "ST_DISTANCE") || !strings.Contains(items.lastQuery, "c.radiusMetres") {
		t.Errorf("query must keep the ST_DISTANCE radius test alongside the authority filter: %q", items.lastQuery)
	}
}

func TestCosmosStore_FindZonesContaining_HybridIndexServedWithLegacyFallback(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	store := NewCosmosStore(items)

	if _, err := store.FindZonesContaining(context.Background(), 99, 51.5, -0.1); err != nil {
		t.Fatalf("FindZonesContaining: %v", err)
	}
	// The primary clause distances against the persisted GeoJSON c.location path so
	// the spatial index on /location (tc-quqe) serves it — this is the perf win.
	if !strings.Contains(items.lastQuery, "ST_DISTANCE(c.location,") {
		t.Errorf("query must distance against the indexed c.location path: %q", items.lastQuery)
	}
	// Any zone written before the location backfill (tc-xj48) has no c.location, so
	// the index-served clause cannot match it. The legacy fallback keeps it matching
	// via the inline [c.longitude, c.latitude] point, guarded by NOT IS_DEFINED so
	// the two clauses never double-count and the switch is correct regardless of
	// backfill state (mirrors the dormant-sweep guard, profiles/admin_store.go).
	if !strings.Contains(items.lastQuery, "NOT IS_DEFINED(c.location)") {
		t.Errorf("query must guard the legacy fallback with NOT IS_DEFINED(c.location): %q", items.lastQuery)
	}
	if !strings.Contains(items.lastQuery, "[c.longitude, c.latitude]") {
		t.Errorf("legacy fallback must distance against the inline [c.longitude, c.latitude] point: %q", items.lastQuery)
	}
	// The authority pre-filter (tc-8dud) survives the hybrid rewrite.
	if !strings.Contains(items.lastQuery, "c.authorityId = @authorityId") {
		t.Errorf("hybrid query must keep the authority pre-filter: %q", items.lastQuery)
	}
}

func TestCosmosStore_FindZonesContaining_ProjectsNeededFieldsNotSelectStar(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	store := NewCosmosStore(items)

	if _, err := store.FindZonesContaining(context.Background(), 99, 51.5, -0.1); err != nil {
		t.Fatalf("FindZonesContaining: %v", err)
	}
	// SELECT * hydrates whole docs on every cross-partition fan-out row. The query
	// instead projects only the columns this path needs: id/userId/createdAt are
	// consumed by the callers; name/radiusMetres/authorityId are required by the
	// NewWatchZone constructor so the hydrated zone stays valid; pushEnabled and
	// emailInstantEnabled are KEPT (not dropped) because they are nullable *bool
	// that coalesce to true when absent — projecting them away would silently
	// re-enable a user's disabled notifications if a future caller read them.
	// latitude/longitude are dropped: no caller reads zone coordinates after the
	// match (the distance test already ran server-side).
	if strings.Contains(items.lastQuery, "SELECT *") {
		t.Errorf("query must not SELECT * on the poll hot path: %q", items.lastQuery)
	}
	for _, field := range []string{"c.id", "c.userId", "c.name", "c.radiusMetres", "c.authorityId", "c.createdAt", "c.pushEnabled", "c.emailInstantEnabled"} {
		if !strings.Contains(items.lastQuery, field) {
			t.Errorf("projection missing %q: %q", field, items.lastQuery)
		}
	}
}

func TestCosmosStore_FindZonesContaining_HydratesProjectedDocument(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	// A projected row carries no latitude/longitude (those columns are dropped from
	// the projection). The store must still hydrate it: coordinates coalesce to the
	// zero value (unused post-match) while the validated fields and the consumed
	// fields (id, userId, createdAt) survive intact, and the stored push/email
	// flags are preserved rather than coalescing to true.
	items.crossPartitionResult = [][]byte{[]byte(`{"id":"zone-9","userId":"auth0|carol","name":"Z","radiusMetres":500,"authorityId":99,"createdAt":"2026-06-01T09:00:00+00:00","pushEnabled":false,"emailInstantEnabled":false}`)}
	store := NewCosmosStore(items)

	zones, err := store.FindZonesContaining(context.Background(), 99, 51.5, -0.1)
	if err != nil {
		t.Fatalf("FindZonesContaining: %v", err)
	}
	if len(zones) != 1 {
		t.Fatalf("zones: got %d, want 1", len(zones))
	}
	got := zones[0]
	if got.ID != "zone-9" || got.UserID != "auth0|carol" {
		t.Errorf("identity: got id=%q user=%q", got.ID, got.UserID)
	}
	if got.CreatedAt.IsZero() {
		t.Error("createdAt must hydrate from the projection, got zero")
	}
	if got.PushEnabled || got.EmailInstantEnabled {
		t.Errorf("stored-false flags must survive the projection: push=%v email=%v", got.PushEnabled, got.EmailInstantEnabled)
	}
}

func TestCosmosStore_FindZonesContaining_EmptyResultIsEmptySlice(t *testing.T) {
	t.Parallel()
	store := NewCosmosStore(newFakeItems())
	zones, err := store.FindZonesContaining(context.Background(), 99, 51.5, -0.1)
	if err != nil {
		t.Fatalf("FindZonesContaining: %v", err)
	}
	if len(zones) != 0 {
		t.Errorf("expected empty, got %+v", zones)
	}
}

func TestCosmosStore_DistinctAuthorityIDs_EmptyQueryYieldsEmpty(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	store := NewCosmosStore(items)
	ids, err := store.DistinctAuthorityIDs(context.Background())
	if err != nil {
		t.Fatalf("DistinctAuthorityIDs: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty, got %v", ids)
	}
}

// legacyDocWithoutLocation is a raw WatchZones document written before the
// GeoJSON write path (tc-x8w9) existed: it carries the authoritative latitude /
// longitude floats and every other field, but no "location" field at all (so
// watchZoneDocument.Location unmarshals to nil). The backfill must rewrite it
// with a derived location while preserving every other column.
func legacyDocWithoutLocation(id, userID string, lat, lon float64, radius float64, authorityID int) []byte {
	return []byte(fmt.Sprintf(
		`{"id":%q,"userId":%q,"name":"Legacy","latitude":%g,"longitude":%g,"radiusMetres":%g,"authorityId":%d,"createdAt":"2026-06-01T09:00:00+00:00","pushEnabled":true,"emailInstantEnabled":true}`,
		id, userID, lat, lon, radius, authorityID))
}

func TestCosmosStore_BackfillLocation_RewritesOnlyDocsMissingLocation(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	// One legacy doc lacks location; two modern docs already carry it (written via
	// newWatchZoneDocument, which always sets location). The scan is cross-partition.
	modernA := testZone(t)
	modernA.ID, modernA.UserID = "modern-a", "auth0|a"
	modernB := testZone(t)
	modernB.ID, modernB.UserID = "modern-b", "auth0|b"
	items.crossPartitionResult = [][]byte{
		mustDocBytes(t, modernA),
		legacyDocWithoutLocation("legacy-1", "auth0|legacy", 53.4808, -2.2426, 750, 471),
		mustDocBytes(t, modernB),
	}
	store := NewCosmosStore(items)

	res, err := store.BackfillLocation(context.Background())
	if err != nil {
		t.Fatalf("BackfillLocation: %v", err)
	}

	if res.Total != 3 || res.AlreadyHad != 2 || res.Backfilled != 1 {
		t.Fatalf("result = %+v, want {Total:3 Backfilled:1 AlreadyHad:2}", res)
	}
	// Exactly one upsert: the legacy doc only. The two modern docs are skipped.
	if len(items.upserts) != 1 {
		t.Fatalf("upserts = %d, want exactly 1 (the legacy doc)", len(items.upserts))
	}
	if items.lastUpsertPK != "auth0|legacy" {
		t.Errorf("upsert partition key = %q, want auth0|legacy", items.lastUpsertPK)
	}

	var doc watchZoneDocument
	if err := json.Unmarshal(items.upserts[0], &doc); err != nil {
		t.Fatalf("unmarshal upserted doc: %v", err)
	}
	// The rewritten doc now carries a GeoJSON location derived from the floats, in
	// [longitude, latitude] order, never recomputed.
	if doc.Location == nil {
		t.Fatal("upserted doc must carry a location")
	}
	if doc.Location.Type != "Point" || len(doc.Location.Coordinates) != 2 ||
		doc.Location.Coordinates[0] != -2.2426 || doc.Location.Coordinates[1] != 53.4808 {
		t.Errorf("location = %+v, want Point[-2.2426, 53.4808]", doc.Location)
	}
	// The legacy doc's other fields survive the rewrite.
	if doc.ID != "legacy-1" || doc.UserID != "auth0|legacy" || doc.RadiusMetres != 750 || doc.AuthorityID != 471 {
		t.Errorf("legacy fields not preserved: %+v", doc)
	}
}

func TestCosmosStore_BackfillLocation_Idempotent(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	// Every doc already carries a location (the steady state after a first run).
	a := testZone(t)
	a.ID, a.UserID = "z-a", "auth0|a"
	b := testZone(t)
	b.ID, b.UserID = "z-b", "auth0|b"
	items.crossPartitionResult = [][]byte{mustDocBytes(t, a), mustDocBytes(t, b)}
	store := NewCosmosStore(items)

	res, err := store.BackfillLocation(context.Background())
	if err != nil {
		t.Fatalf("BackfillLocation: %v", err)
	}
	if res.Total != 2 || res.AlreadyHad != 2 || res.Backfilled != 0 {
		t.Fatalf("result = %+v, want {Total:2 Backfilled:0 AlreadyHad:2}", res)
	}
	if len(items.upserts) != 0 {
		t.Errorf("a re-run must write nothing, got %d upserts", len(items.upserts))
	}
}

func TestCosmosStore_BackfillLocation_QueryError(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	items.queryErr = errors.New("gateway boom")
	store := NewCosmosStore(items)

	if _, err := store.BackfillLocation(context.Background()); err == nil {
		t.Fatal("expected an error when the cross-partition scan fails")
	}
}

func TestCosmosStore_BackfillLocation_EmptyContainerIsZeroResult(t *testing.T) {
	t.Parallel()
	store := NewCosmosStore(newFakeItems())
	res, err := store.BackfillLocation(context.Background())
	if err != nil {
		t.Fatalf("BackfillLocation: %v", err)
	}
	if res.Total != 0 || res.Backfilled != 0 || res.AlreadyHad != 0 {
		t.Errorf("result = %+v, want zero", res)
	}
}
