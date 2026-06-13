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
	lastQueryPK  string
	lastQuery    string
	lastParams   map[string]any
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
	return f.deleteErr
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
