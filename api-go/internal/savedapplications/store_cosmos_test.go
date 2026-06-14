package savedapplications

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

type fakeItems struct {
	stored               map[string][]byte
	deleteErr            error
	queryResult          [][]byte
	crossPartitionResult [][]byte
	lastUpsertPK         string
	lastDeleteID         string
	lastDeletePK         string
	deletedIDs           []string
	lastQueryPK          string
	lastQuery            string
	lastParams           map[string]any
}

func newFakeItems() *fakeItems { return &fakeItems{stored: map[string][]byte{}} }

func (f *fakeItems) ReadItem(_ context.Context, _, id string) ([]byte, error) {
	b, ok := f.stored[id]
	if !ok {
		return nil, &azcore.ResponseError{StatusCode: http.StatusNotFound}
	}
	return b, nil
}

func (f *fakeItems) UpsertItem(_ context.Context, partitionKey string, item []byte) error {
	f.lastUpsertPK = partitionKey
	var doc struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(item, &doc)
	f.stored[doc.ID] = item
	return nil
}

func (f *fakeItems) DeleteItem(_ context.Context, partitionKey, id string) error {
	f.lastDeleteID = id
	f.lastDeletePK = partitionKey
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
	return f.queryResult, nil
}

func (f *fakeItems) QueryItemsCrossPartition(_ context.Context, query string, params map[string]any) ([][]byte, error) {
	f.lastQuery = query
	f.lastParams = params
	return f.crossPartitionResult, nil
}

func savedFixture(t *testing.T) SavedApplication {
	t.Helper()
	return NewSavedApplication("auth0|u", testApp(t), time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))
}

func TestCosmosStore_Save_UpsertsToUserPartition(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	store := NewCosmosStore(items)
	s := savedFixture(t)

	if err := store.Save(context.Background(), s); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if items.lastUpsertPK != "auth0|u" {
		t.Errorf("upsert pk: got %q", items.lastUpsertPK)
	}
	if _, ok := items.stored["auth0|u:471/24/0123/FUL"]; !ok {
		t.Errorf("doc not stored under composite id, have %v", keys(items.stored))
	}
}

func TestCosmosStore_Exists(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	store := NewCosmosStore(items)
	s := savedFixture(t)
	_ = store.Save(context.Background(), s)

	ok, err := store.Exists(context.Background(), "auth0|u", "471/24/0123/FUL")
	if err != nil || !ok {
		t.Fatalf("Exists existing: ok=%v err=%v", ok, err)
	}
	ok, err = store.Exists(context.Background(), "auth0|u", "nope")
	if err != nil || ok {
		t.Fatalf("Exists missing: ok=%v err=%v", ok, err)
	}
}

func TestCosmosStore_Delete_SwallowsNotFound(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	items.deleteErr = &azcore.ResponseError{StatusCode: http.StatusNotFound}
	store := NewCosmosStore(items)

	if err := store.Delete(context.Background(), "auth0|u", "missing"); err != nil {
		t.Fatalf("Delete of missing must be a no-op, got %v", err)
	}
	if items.lastDeleteID != "auth0|u:missing" {
		t.Errorf("delete id: got %q", items.lastDeleteID)
	}
}

func TestCosmosStore_GetByUserID(t *testing.T) {
	t.Parallel()
	s := savedFixture(t)
	body, _ := json.Marshal(newSavedApplicationDocument(s))
	items := newFakeItems()
	items.queryResult = [][]byte{body}
	store := NewCosmosStore(items)

	got, err := store.GetByUserID(context.Background(), "auth0|u")
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if len(got) != 1 || got[0].ApplicationUID != "471/24/0123/FUL" {
		t.Fatalf("got %+v", got)
	}
	if items.lastQueryPK != "auth0|u" || items.lastParams["@userId"] != "auth0|u" {
		t.Errorf("query routing: pk=%q params=%v", items.lastQueryPK, items.lastParams)
	}
}

func TestCosmosStore_DeleteAllByUserID_QueriesPartitionThenDeletesEachID(t *testing.T) {
	t.Parallel()
	const userID = "auth0|u"
	items := newFakeItems()
	items.queryResult = [][]byte{
		[]byte(`{"id":"auth0|u:app-a"}`),
		[]byte(`{"id":"auth0|u:app-b"}`),
	}
	store := NewCosmosStore(items)

	if err := store.DeleteAllByUserID(context.Background(), userID); err != nil {
		t.Fatalf("DeleteAllByUserID: %v", err)
	}
	if items.lastQueryPK != userID {
		t.Errorf("cascade query pk: got %q, want %q", items.lastQueryPK, userID)
	}
	if len(items.deletedIDs) != 2 || items.deletedIDs[0] != "auth0|u:app-a" || items.deletedIDs[1] != "auth0|u:app-b" {
		t.Errorf("deleted ids: got %v", items.deletedIDs)
	}
	if items.lastDeletePK != userID {
		t.Errorf("cascade delete pk: got %q, want %q", items.lastDeletePK, userID)
	}
}

func TestCosmosStore_UserIDsForApplication_CrossPartitionScopedToAuthority(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	// SELECT VALUE c.userId yields bare JSON strings.
	items.crossPartitionResult = [][]byte{[]byte(`"auth0|alice"`), []byte(`"auth0|bob"`)}
	store := NewCosmosStore(items)

	userIDs, err := store.UserIDsForApplication(context.Background(), "24/0001", 99)
	if err != nil {
		t.Fatalf("UserIDsForApplication: %v", err)
	}
	if len(userIDs) != 2 || userIDs[0] != "auth0|alice" || userIDs[1] != "auth0|bob" {
		t.Fatalf("user ids: got %v", userIDs)
	}
	// Predicate must be scoped to the authority: PlanIt uids collide across
	// councils (tc-th98), so a uid-only match would falsely notify a Bradford
	// bookmark holder about a Kingston decision.
	if items.lastParams["@applicationUid"] != "24/0001" || items.lastParams["@authorityId"] != 99 {
		t.Errorf("params: got uid=%v authority=%v", items.lastParams["@applicationUid"], items.lastParams["@authorityId"])
	}
}

func TestCosmosStore_UserIDsForApplication_EmptyResultIsEmptySlice(t *testing.T) {
	t.Parallel()
	store := NewCosmosStore(newFakeItems())
	userIDs, err := store.UserIDsForApplication(context.Background(), "24/0001", 99)
	if err != nil {
		t.Fatalf("UserIDsForApplication: %v", err)
	}
	if len(userIDs) != 0 {
		t.Errorf("expected empty, got %v", userIDs)
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
