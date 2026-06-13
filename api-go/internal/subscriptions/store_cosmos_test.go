package subscriptions

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// fakeNotificationItems is a hand-written notificationItems: point reads answer
// from a fixed map, and a missing key surfaces as an azcore 404.
type fakeNotificationItems struct {
	docs     map[string][]byte
	readErr  error
	upserted map[string][]byte
}

func newFakeNotificationItems() *fakeNotificationItems {
	return &fakeNotificationItems{docs: map[string][]byte{}, upserted: map[string][]byte{}}
}

func (f *fakeNotificationItems) ReadItem(_ context.Context, _, id string) ([]byte, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	raw, ok := f.docs[id]
	if !ok {
		return nil, &azcore.ResponseError{StatusCode: http.StatusNotFound}
	}
	return raw, nil
}

func (f *fakeNotificationItems) UpsertItem(_ context.Context, partitionKey string, item []byte) error {
	f.upserted[partitionKey] = item
	return nil
}

func TestCosmosNotificationStore_IsProcessed_NotFound(t *testing.T) {
	t.Parallel()
	store := NewCosmosNotificationStore(newFakeNotificationItems(), time.Now)

	processed, err := store.IsProcessed(context.Background(), "uuid-1")
	if err != nil {
		t.Fatalf("IsProcessed: %v", err)
	}
	if processed {
		t.Error("want not processed for an absent uuid")
	}
}

func TestCosmosNotificationStore_IsProcessed_Found(t *testing.T) {
	t.Parallel()
	items := newFakeNotificationItems()
	items.docs["uuid-1"] = []byte(`{"id":"uuid-1"}`)
	store := NewCosmosNotificationStore(items, time.Now)

	processed, err := store.IsProcessed(context.Background(), "uuid-1")
	if err != nil {
		t.Fatalf("IsProcessed: %v", err)
	}
	if !processed {
		t.Error("want processed for a present uuid")
	}
}

func TestCosmosNotificationStore_MarkProcessed(t *testing.T) {
	t.Parallel()
	items := newFakeNotificationItems()
	store := NewCosmosNotificationStore(items, func() time.Time {
		return time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	})

	if err := store.MarkProcessed(context.Background(), "uuid-7"); err != nil {
		t.Fatalf("MarkProcessed: %v", err)
	}

	raw, ok := items.upserted["uuid-7"]
	if !ok {
		t.Fatal("notification not upserted under its uuid partition key")
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal doc: %v", err)
	}
	if doc["id"] != "uuid-7" {
		t.Errorf("doc id = %v, want uuid-7", doc["id"])
	}
	if _, ok := doc["processedAt"]; !ok {
		t.Error("doc missing processedAt")
	}
}
