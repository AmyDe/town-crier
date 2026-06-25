package watchzones

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

// legacyDocWithoutBoundingBox builds a document that predates the bounding-box
// write path (tc-b179): it may already carry a GeoJSON location (a doc that was
// only location-backfilled), but it has no minLat/maxLat/minLon/maxLon, which is
// exactly the "needs bbox backfill" signal BackfillBoundingBox keys on.
func legacyDocWithoutBoundingBox(id, userID string, lat, lon, radius float64, authorityID int) []byte {
	return []byte(fmt.Sprintf(
		`{"id":%q,"userId":%q,"name":"Legacy","latitude":%g,"longitude":%g,"radiusMetres":%g,"authorityId":%d,"location":{"type":"Point","coordinates":[%g,%g]},"createdAt":"2026-06-01T09:00:00+00:00","pushEnabled":true,"emailInstantEnabled":true}`,
		id, userID, lat, lon, radius, authorityID, lon, lat))
}

func TestCosmosStore_BackfillBoundingBox_RewritesOnlyDocsMissingBoundingBox(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	// One legacy doc lacks a bounding box (location-only, predating tc-b179); two
	// modern docs already carry it (written via newWatchZoneDocument, which always
	// computes the bbox). The scan is cross-partition.
	modernA := testZone(t)
	modernA.ID, modernA.UserID = "modern-a", "auth0|a"
	modernB := testZone(t)
	modernB.ID, modernB.UserID = "modern-b", "auth0|b"
	items.crossPartitionResult = [][]byte{
		mustDocBytes(t, modernA),
		legacyDocWithoutBoundingBox("legacy-1", "auth0|legacy", 53.4808, -2.2426, 750, 471),
		mustDocBytes(t, modernB),
	}
	store := NewCosmosStore(items)

	res, err := store.BackfillBoundingBox(context.Background())
	if err != nil {
		t.Fatalf("BackfillBoundingBox: %v", err)
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
	// The rewritten doc now carries a bounding box derived from its centre+radius,
	// matching what boundingBox() computes for the same zone.
	wantMinLat, wantMaxLat, wantMinLon, wantMaxLon := boundingBoxFor(t, 53.4808, -2.2426, 750, 471)
	if doc.MinLat == nil || doc.MaxLat == nil || doc.MinLon == nil || doc.MaxLon == nil {
		t.Fatalf("upserted doc must carry a bounding box, got %+v", doc)
	}
	if *doc.MinLat != wantMinLat || *doc.MaxLat != wantMaxLat || *doc.MinLon != wantMinLon || *doc.MaxLon != wantMaxLon {
		t.Errorf("bbox = [%g,%g,%g,%g], want [%g,%g,%g,%g]",
			*doc.MinLat, *doc.MaxLat, *doc.MinLon, *doc.MaxLon, wantMinLat, wantMaxLat, wantMinLon, wantMaxLon)
	}
	// The legacy doc's other fields survive the rewrite.
	if doc.ID != "legacy-1" || doc.UserID != "auth0|legacy" || doc.RadiusMetres != 750 || doc.AuthorityID != 471 {
		t.Errorf("legacy fields not preserved: %+v", doc)
	}
}

func TestCosmosStore_BackfillBoundingBox_Idempotent(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	// Every doc already carries a bounding box (the steady state after a first run).
	a := testZone(t)
	a.ID, a.UserID = "z-a", "auth0|a"
	b := testZone(t)
	b.ID, b.UserID = "z-b", "auth0|b"
	items.crossPartitionResult = [][]byte{mustDocBytes(t, a), mustDocBytes(t, b)}
	store := NewCosmosStore(items)

	res, err := store.BackfillBoundingBox(context.Background())
	if err != nil {
		t.Fatalf("BackfillBoundingBox: %v", err)
	}
	if res.Total != 2 || res.AlreadyHad != 2 || res.Backfilled != 0 {
		t.Fatalf("result = %+v, want {Total:2 Backfilled:0 AlreadyHad:2}", res)
	}
	if len(items.upserts) != 0 {
		t.Errorf("a re-run must write nothing, got %d upserts", len(items.upserts))
	}
}

func TestCosmosStore_BackfillBoundingBox_QueryError(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	items.queryErr = errors.New("gateway boom")
	store := NewCosmosStore(items)

	if _, err := store.BackfillBoundingBox(context.Background()); err == nil {
		t.Fatal("expected an error when the cross-partition scan fails")
	}
}

func TestCosmosStore_BackfillBoundingBox_EmptyContainerIsZeroResult(t *testing.T) {
	t.Parallel()
	store := NewCosmosStore(newFakeItems())
	res, err := store.BackfillBoundingBox(context.Background())
	if err != nil {
		t.Fatalf("BackfillBoundingBox: %v", err)
	}
	if res.Total != 0 || res.Backfilled != 0 || res.AlreadyHad != 0 {
		t.Errorf("result = %+v, want zero", res)
	}
}

// boundingBoxFor computes the expected bounding box for a zone's centre+radius
// via the same domain helper the write path uses, so the test asserts against
// the production formula rather than a hand-copied literal.
func boundingBoxFor(t *testing.T, lat, lon, radius float64, authorityID int) (minLat, maxLat, minLon, maxLon float64) {
	t.Helper()
	z, err := NewWatchZone("bbox", "u", "Box", lat, lon, radius, authorityID,
		testZone(t).CreatedAt, true, true)
	if err != nil {
		t.Fatalf("NewWatchZone: %v", err)
	}
	return z.boundingBox()
}
