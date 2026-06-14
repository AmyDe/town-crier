package devicetokens

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// fakeItems is an in-memory CosmosItems keyed by (partitionKey, id). It records
// the last upsert partition key so tests can assert single-partition scoping.
type fakeItems struct {
	docs         map[string][]byte // key: partitionKey + "\x00" + id
	upsertPart   string
	upsertErr    error
	deleteErr    error
	readErr      error
	queryErr     error
	queryResult  [][]byte // when set, returned verbatim by QueryItems
	deleteCalls  int
	lastQueryPK  string
	lastDeletePK string
	deletedIDs   []string
}

func newFakeItems() *fakeItems { return &fakeItems{docs: map[string][]byte{}} }

func key(pk, id string) string { return pk + "\x00" + id }

func (f *fakeItems) ReadItem(_ context.Context, pk, id string) ([]byte, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	raw, ok := f.docs[key(pk, id)]
	if !ok {
		return nil, notFound()
	}
	return raw, nil
}

func (f *fakeItems) UpsertItem(_ context.Context, pk string, item []byte) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.upsertPart = pk
	var doc struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(item, &doc)
	f.docs[key(pk, doc.ID)] = item
	return nil
}

func (f *fakeItems) DeleteItem(_ context.Context, pk, id string) error {
	f.deleteCalls++
	f.lastDeletePK = pk
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deletedIDs = append(f.deletedIDs, id)
	delete(f.docs, key(pk, id))
	return nil
}

func (f *fakeItems) QueryItems(_ context.Context, pk, _ string, _ map[string]any) ([][]byte, error) {
	f.lastQueryPK = pk
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	if f.queryResult != nil {
		return f.queryResult, nil
	}
	var out [][]byte
	prefix := pk + "\x00"
	for k, v := range f.docs {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			out = append(out, v)
		}
	}
	return out, nil
}

func notFound() error { return &azcore.ResponseError{StatusCode: http.StatusNotFound} }

func TestCosmosStore_SaveThenGetByToken(t *testing.T) {
	t.Parallel()

	items := newFakeItems()
	store := NewCosmosStore(items)
	ctx := context.Background()
	now := time.Date(2026, 6, 12, 9, 0, 0, 0, time.UTC)

	reg, err := NewRegistration("auth0|u1", "tok-abc", PlatformIos, now)
	if err != nil {
		t.Fatalf("NewRegistration: %v", err)
	}
	if err := store.Save(ctx, reg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if items.upsertPart != "auth0|u1" {
		t.Errorf("upsert partition = %q, want the user id", items.upsertPart)
	}

	got, err := store.GetByToken(ctx, "auth0|u1", "tok-abc")
	if err != nil {
		t.Fatalf("GetByToken: %v", err)
	}
	if got == nil {
		t.Fatal("GetByToken: got nil, want the saved registration")
	}
	if got.Token != "tok-abc" || got.Platform != PlatformIos {
		t.Errorf("GetByToken returned %+v", got)
	}
}

// TestCosmosStore_GetByToken_Missing returns (nil, nil) for an absent token,
// mirroring .NET's null return — the handler's existence check, not an error.
func TestCosmosStore_GetByToken_Missing(t *testing.T) {
	t.Parallel()

	store := NewCosmosStore(newFakeItems())
	got, err := store.GetByToken(context.Background(), "auth0|u1", "missing")
	if err != nil {
		t.Fatalf("GetByToken missing: got err %v, want nil", err)
	}
	if got != nil {
		t.Errorf("GetByToken missing: got %+v, want nil", got)
	}
}

// TestCosmosStore_Delete_Idempotent: a 404 on delete is tolerated (the token was
// already removed by a prior call or the TTL), matching .NET's idempotent delete.
func TestCosmosStore_Delete_Idempotent(t *testing.T) {
	t.Parallel()

	items := newFakeItems()
	items.deleteErr = notFound()
	store := NewCosmosStore(items)

	if err := store.Delete(context.Background(), "auth0|u1", "gone"); err != nil {
		t.Errorf("Delete on absent token: got err %v, want nil (idempotent)", err)
	}
}

// TestCosmosStore_Delete_PropagatesRealError: a non-404 delete failure is
// surfaced, not swallowed.
func TestCosmosStore_Delete_PropagatesRealError(t *testing.T) {
	t.Parallel()

	items := newFakeItems()
	items.deleteErr = errors.New("cosmos down")
	store := NewCosmosStore(items)

	if err := store.Delete(context.Background(), "auth0|u1", "tok"); err == nil {
		t.Error("Delete with a real failure: want error")
	}
}

func TestCosmosStore_ListByUser(t *testing.T) {
	t.Parallel()

	items := newFakeItems()
	store := NewCosmosStore(items)
	ctx := context.Background()
	now := time.Date(2026, 6, 12, 9, 0, 0, 0, time.UTC)

	for _, tok := range []string{"t1", "t2"} {
		reg, _ := NewRegistration("auth0|u1", tok, PlatformIos, now)
		if err := store.Save(ctx, reg); err != nil {
			t.Fatalf("Save %s: %v", tok, err)
		}
	}
	other, _ := NewRegistration("auth0|other", "t3", PlatformAndroid, now)
	if err := store.Save(ctx, other); err != nil {
		t.Fatalf("Save other: %v", err)
	}

	got, err := store.ListByUser(ctx, "auth0|u1")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("ListByUser returned %d registrations, want 2", len(got))
	}
}

func TestCosmosStore_DeleteAllByUserID_QueriesPartitionThenDeletesEachID(t *testing.T) {
	t.Parallel()
	const userID = "auth0|u"
	items := newFakeItems()
	items.queryResult = [][]byte{
		[]byte(`{"id":"token-a"}`),
		[]byte(`{"id":"token-b"}`),
	}
	store := NewCosmosStore(items)

	if err := store.DeleteAllByUserID(context.Background(), userID); err != nil {
		t.Fatalf("DeleteAllByUserID: %v", err)
	}
	if items.lastQueryPK != userID {
		t.Errorf("cascade query pk: got %q, want %q", items.lastQueryPK, userID)
	}
	if len(items.deletedIDs) != 2 || items.deletedIDs[0] != "token-a" || items.deletedIDs[1] != "token-b" {
		t.Errorf("deleted ids: got %v", items.deletedIDs)
	}
	if items.lastDeletePK != userID {
		t.Errorf("cascade delete pk: got %q, want %q", items.lastDeletePK, userID)
	}
}
