package profiles

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// fakeItems is a hand-written stand-in for the azcosmos container item API.
// It records the partition key and id of each call so the store's point-read
// behaviour can be asserted, and lets a test inject a stored document or a
// specific error.
type fakeItems struct {
	stored      map[string][]byte // id -> raw document bytes
	readErr     error
	upsertErr   error
	deleteErr   error
	lastReadPK  string
	lastReadID  string
	lastUpsert  []byte
	lastDeleted string
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
	f.lastUpsert = item
	return f.upsertErr
}

func (f *fakeItems) DeleteItem(_ context.Context, partitionKey, id string) error {
	f.lastDeleted = id
	return f.deleteErr
}

func storeWithProfile(t *testing.T, p *UserProfile) (*CosmosStore, *fakeItems) {
	t.Helper()
	items := newFakeItems()
	b, err := json.Marshal(newProfileDocument(p))
	if err != nil {
		t.Fatalf("marshal seed: %v", err)
	}
	items.stored[p.UserID] = b
	return NewCosmosStore(items), items
}

func TestCosmosStore_Get_PointReadsByUserID(t *testing.T) {
	t.Parallel()

	p, _ := NewProfile("auth0|abc", "user@example.com", time.Now().UTC().Truncate(time.Second))
	store, items := storeWithProfile(t, p)

	got, err := store.Get(context.Background(), "auth0|abc")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.UserID != "auth0|abc" {
		t.Errorf("UserID: got %q", got.UserID)
	}
	// Point read: partition key and id both equal the user id.
	if items.lastReadPK != "auth0|abc" || items.lastReadID != "auth0|abc" {
		t.Errorf("point read used pk=%q id=%q, want both auth0|abc", items.lastReadPK, items.lastReadID)
	}
}

func TestCosmosStore_Get_NotFound(t *testing.T) {
	t.Parallel()

	store := NewCosmosStore(newFakeItems())
	_, err := store.Get(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get missing: got %v, want ErrNotFound", err)
	}
}

func TestCosmosStore_Get_PropagatesOtherErrors(t *testing.T) {
	t.Parallel()

	items := newFakeItems()
	items.readErr = &azcore.ResponseError{StatusCode: http.StatusServiceUnavailable}
	store := NewCosmosStore(items)

	_, err := store.Get(context.Background(), "u1")
	if err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("503 should surface as a non-NotFound error, got %v", err)
	}
}

func TestCosmosStore_Save_UpsertsDocument(t *testing.T) {
	t.Parallel()

	items := newFakeItems()
	store := NewCosmosStore(items)
	p, _ := NewProfile("u1", "user@example.com", time.Now().UTC().Truncate(time.Second))

	if err := store.Save(context.Background(), p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(items.lastUpsert, &doc); err != nil {
		t.Fatalf("unmarshal upserted: %v", err)
	}
	if doc["id"] != "u1" || doc["userId"] != "u1" {
		t.Errorf("upserted doc id/userId wrong: %v", doc)
	}
}

func TestCosmosStore_Delete_RemovesByUserID(t *testing.T) {
	t.Parallel()

	items := newFakeItems()
	store := NewCosmosStore(items)
	if err := store.Delete(context.Background(), "u1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if items.lastDeleted != "u1" {
		t.Errorf("deleted id: got %q, want u1", items.lastDeleted)
	}
}

func TestCosmosStore_Delete_NotFoundTolerant(t *testing.T) {
	t.Parallel()

	items := newFakeItems()
	items.deleteErr = &azcore.ResponseError{StatusCode: http.StatusNotFound}
	store := NewCosmosStore(items)

	// A 404 on delete surfaces as ErrNotFound so the caller (DELETE /v1/me) can
	// translate it to a 404 status, mirroring .NET's UserProfileNotFoundException
	// path. The store itself does not swallow it.
	if err := store.Delete(context.Background(), "u1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete 404: got %v, want ErrNotFound", err)
	}
}
