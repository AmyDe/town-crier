package watchzones

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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
	return f.upsertErr
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
	// SELECT DISTINCT VALUE c.authorityId FROM c yields bare JSON numbers.
	items.crossPartitionResult = [][]byte{[]byte("99"), []byte("123"), []byte("7")}
	store := NewCosmosStore(items)

	ids, err := store.DistinctAuthorityIDs(context.Background())
	if err != nil {
		t.Fatalf("DistinctAuthorityIDs: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("ids: got %v, want 3 entries", ids)
	}
	got := map[int]bool{}
	for _, id := range ids {
		got[id] = true
	}
	for _, want := range []int{99, 123, 7} {
		if !got[want] {
			t.Errorf("missing authority id %d in %v", want, ids)
		}
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
