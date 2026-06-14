package notificationstate

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

type fakeItems struct {
	docs         map[string][]byte
	deleteErr    error
	lastDeletePK string
	lastDeleteID string
}

func (f *fakeItems) ReadItem(_ context.Context, _, id string) ([]byte, error) {
	raw, ok := f.docs[id]
	if !ok {
		return nil, &azcore.ResponseError{StatusCode: http.StatusNotFound}
	}
	return raw, nil
}

func (f *fakeItems) UpsertItem(_ context.Context, _ string, item []byte) error {
	if f.docs == nil {
		f.docs = map[string][]byte{}
	}
	f.docs[idOf(item)] = item
	return nil
}

func (f *fakeItems) DeleteItem(_ context.Context, partitionKey, id string) error {
	f.lastDeletePK = partitionKey
	f.lastDeleteID = id
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.docs, id)
	return nil
}

func idOf(raw []byte) string {
	var doc stateDocument
	_ = json.Unmarshal(raw, &doc)
	return doc.ID
}

type fakeCounter struct {
	count     int
	lastQuery string
	params    map[string]any
}

func (f *fakeCounter) CountItems(_ context.Context, _, query string, params map[string]any) (int, error) {
	f.lastQuery = query
	f.params = params
	return f.count, nil
}

func TestCosmosStore_RoundTrip(t *testing.T) {
	t.Parallel()

	items := &fakeItems{}
	store := NewCosmosStore(items, &fakeCounter{})

	missing, err := store.Get(context.Background(), "auth0|s1")
	if err != nil || missing != nil {
		t.Fatalf("missing state: got (%v, %v), want (nil, nil)", missing, err)
	}

	st := State{UserID: "auth0|s1", LastReadAt: time.Date(2026, 6, 12, 9, 0, 0, 0, time.UTC), Version: 4}
	if err := store.Save(context.Background(), st); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Stored shape pins the .NET document contract.
	want := `{"id":"auth0|s1","userId":"auth0|s1","lastReadAt":"2026-06-12T09:00:00+00:00","version":4}`
	if got := string(items.docs["auth0|s1"]); got != want {
		t.Errorf("stored doc = %s, want %s", got, want)
	}

	loaded, err := store.Get(context.Background(), "auth0|s1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if loaded.Version != 4 || !loaded.LastReadAt.Equal(st.LastReadAt) {
		t.Errorf("loaded = %+v, want %+v", loaded, st)
	}
}

func TestCosmosStore_UnreadCount_CutoffFormat(t *testing.T) {
	t.Parallel()

	counter := &fakeCounter{count: 5}
	store := NewCosmosStore(&fakeItems{}, counter)

	got, err := store.UnreadCount(context.Background(), "auth0|s1",
		time.Date(2026, 6, 12, 9, 0, 0, 0, time.UTC))
	if err != nil || got != 5 {
		t.Fatalf("count: got (%d, %v), want (5, nil)", got, err)
	}
	// The cutoff parameter must use the .NET DateTimeOffset string form so the
	// lexicographic comparison against .NET-written createdAt values holds.
	if cutoff := counter.params["@lastReadAt"]; cutoff != "2026-06-12T09:00:00+00:00" {
		t.Errorf("cutoff param = %v, want +00:00 form", cutoff)
	}
}

func TestCosmosStore_DeleteByUserID_PointDeletesUserDocument(t *testing.T) {
	t.Parallel()
	const userID = "auth0|s1"
	items := &fakeItems{docs: map[string][]byte{userID: []byte(`{"id":"auth0|s1"}`)}}
	store := NewCosmosStore(items, &fakeCounter{})

	if err := store.DeleteByUserID(context.Background(), userID); err != nil {
		t.Fatalf("DeleteByUserID: %v", err)
	}
	if items.lastDeletePK != userID || items.lastDeleteID != userID {
		t.Errorf("delete keys: pk=%q id=%q, want both %q", items.lastDeletePK, items.lastDeleteID, userID)
	}
	if _, ok := items.docs[userID]; ok {
		t.Error("document still present after DeleteByUserID")
	}
}

func TestCosmosStore_DeleteByUserID_MissingIsNoError(t *testing.T) {
	t.Parallel()
	items := &fakeItems{deleteErr: &azcore.ResponseError{StatusCode: http.StatusNotFound}}
	store := NewCosmosStore(items, &fakeCounter{})

	if err := store.DeleteByUserID(context.Background(), "auth0|gone"); err != nil {
		t.Errorf("DeleteByUserID on absent watermark: got %v, want nil (idempotent)", err)
	}
}
