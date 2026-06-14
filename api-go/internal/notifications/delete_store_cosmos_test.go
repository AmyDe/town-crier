package notifications

import (
	"context"
	"errors"
	"testing"
)

// fakeDeleteItems is a hand-written stand-in for the query+delete slice of the
// Notifications container the DeleteStore depends on. It records the partition
// key for the cascade query and every deleted id, and can inject errors.
type fakeDeleteItems struct {
	queryResult  [][]byte
	queryErr     error
	deleteErr    error
	lastQueryPK  string
	lastQuery    string
	lastDeletePK string
	deletedIDs   []string
}

func (f *fakeDeleteItems) QueryItems(_ context.Context, partitionKey, query string, _ map[string]any) ([][]byte, error) {
	f.lastQueryPK = partitionKey
	f.lastQuery = query
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return f.queryResult, nil
}

func (f *fakeDeleteItems) DeleteItem(_ context.Context, partitionKey, id string) error {
	f.lastDeletePK = partitionKey
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deletedIDs = append(f.deletedIDs, id)
	return nil
}

func TestDeleteStore_DeleteAllByUserID_QueriesPartitionThenDeletesEachID(t *testing.T) {
	t.Parallel()
	const userID = "auth0|u"
	items := &fakeDeleteItems{queryResult: [][]byte{
		[]byte(`{"id":"notif-a"}`),
		[]byte(`{"id":"notif-b"}`),
	}}
	store := NewDeleteStore(items)

	if err := store.DeleteAllByUserID(context.Background(), userID); err != nil {
		t.Fatalf("DeleteAllByUserID: %v", err)
	}
	if items.lastQueryPK != userID {
		t.Errorf("cascade query pk: got %q, want %q", items.lastQueryPK, userID)
	}
	if len(items.deletedIDs) != 2 || items.deletedIDs[0] != "notif-a" || items.deletedIDs[1] != "notif-b" {
		t.Errorf("deleted ids: got %v", items.deletedIDs)
	}
	if items.lastDeletePK != userID {
		t.Errorf("cascade delete pk: got %q, want %q", items.lastDeletePK, userID)
	}
}

func TestDeleteStore_DeleteAllByUserID_QueryErrorPropagates(t *testing.T) {
	t.Parallel()
	items := &fakeDeleteItems{queryErr: errors.New("cosmos down")}
	store := NewDeleteStore(items)

	if err := store.DeleteAllByUserID(context.Background(), "auth0|u"); err == nil {
		t.Fatal("expected error when the cascade query fails")
	}
}

func TestDeleteStore_DeleteAllByUserID_NoneIsNoOp(t *testing.T) {
	t.Parallel()
	items := &fakeDeleteItems{}
	store := NewDeleteStore(items)

	if err := store.DeleteAllByUserID(context.Background(), "auth0|nobody"); err != nil {
		t.Fatalf("DeleteAllByUserID: %v", err)
	}
	if len(items.deletedIDs) != 0 {
		t.Errorf("expected no deletes, got %v", items.deletedIDs)
	}
}
