package subscriptions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// notificationItems is the consumer-side slice of the AppleNotifications
// container the idempotency store uses: a single-partition point read and
// upsert, both keyed on the Apple notificationUUID. platform.CosmosContainer
// satisfies it structurally.
type notificationItems interface {
	ReadItem(ctx context.Context, partitionKey, id string) ([]byte, error)
	UpsertItem(ctx context.Context, partitionKey string, item []byte) error
}

// processedNotificationDocument records that an App Store Server Notification
// has been handled. The document id and partition key are both the
// notificationUUID, so a duplicate delivery is detected with one point read.
// Mirrors the .NET ProcessedNotificationDocument.
type processedNotificationDocument struct {
	ID          string              `json:"id"`
	ProcessedAt platform.DotNetTime `json:"processedAt"`
}

// CosmosNotificationStore records and detects processed App Store Server
// Notifications in the AppleNotifications container, giving the webhook handler
// at-most-once processing. Mirrors the .NET CosmosNotificationIdempotencyStore.
type CosmosNotificationStore struct {
	items notificationItems
	now   func() time.Time
}

// NewCosmosNotificationStore returns a store backed by the given accessor. now
// supplies the processed-at timestamp (injected for deterministic tests).
func NewCosmosNotificationStore(items notificationItems, now func() time.Time) *CosmosNotificationStore {
	return &CosmosNotificationStore{items: items, now: now}
}

// IsProcessed reports whether the notification has already been handled. A 404
// from Cosmos means "not yet" — not an error.
func (s *CosmosNotificationStore) IsProcessed(ctx context.Context, notificationUUID string) (bool, error) {
	_, err := s.items.ReadItem(ctx, notificationUUID, notificationUUID)
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("read processed notification %q: %w", notificationUUID, err)
	}
	return true, nil
}

// MarkProcessed records the notification as handled. The upsert is
// last-writer-wins so a re-delivery that races past IsProcessed still completes
// without a conflict error.
func (s *CosmosNotificationStore) MarkProcessed(ctx context.Context, notificationUUID string) error {
	doc := processedNotificationDocument{
		ID:          notificationUUID,
		ProcessedAt: platform.DotNetTime(s.now()),
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("encode processed notification %q: %w", notificationUUID, err)
	}
	if err := s.items.UpsertItem(ctx, notificationUUID, body); err != nil {
		return fmt.Errorf("upsert processed notification %q: %w", notificationUUID, err)
	}
	return nil
}

func isNotFound(err error) bool {
	var respErr *azcore.ResponseError
	return errors.As(err, &respErr) && respErr.StatusCode == http.StatusNotFound
}
