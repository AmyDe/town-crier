package notificationstate

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// CosmosItems is the consumer-side slice of the NotificationState container the
// store uses: point read/upsert/delete keyed on the user id (one document per
// user, id == partition key). platform.CosmosContainer satisfies it structurally.
type CosmosItems interface {
	ReadItem(ctx context.Context, partitionKey, id string) ([]byte, error)
	UpsertItem(ctx context.Context, partitionKey string, item []byte) error
	DeleteItem(ctx context.Context, partitionKey, id string) error
}

// CosmosCounter is the consumer-side slice of the Notifications container the
// store uses: a single-partition scalar COUNT query for the unread tally.
type CosmosCounter interface {
	CountItems(ctx context.Context, partitionKey, query string, params map[string]any) (int, error)
}

// CosmosStore reads and writes the per-user watermark in the NotificationState
// container and derives the unread count from the Notifications container.
type CosmosStore struct {
	state         CosmosItems
	notifications CosmosCounter
}

// NewCosmosStore returns a store over the two container accessors.
func NewCosmosStore(state CosmosItems, notifications CosmosCounter) *CosmosStore {
	return &CosmosStore{state: state, notifications: notifications}
}

// Get point-reads the user's watermark. A missing document returns (nil, nil) —
// the first-touch signal the handlers branch on.
func (s *CosmosStore) Get(ctx context.Context, userID string) (*State, error) {
	raw, err := s.state.ReadItem(ctx, userID, userID)
	if err != nil {
		if platform.IsCosmosNotFound(err) {
			return nil, nil //nolint:nilnil // absent watermark is the first-touch signal, not an error
		}
		return nil, fmt.Errorf("read notification state %q: %w", userID, err)
	}
	var doc stateDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decode notification state %q: %w", userID, err)
	}
	st := doc.toDomain()
	return &st, nil
}

// Save upserts the watermark document.
func (s *CosmosStore) Save(ctx context.Context, st State) error {
	body, err := json.Marshal(newStateDocument(st))
	if err != nil {
		return fmt.Errorf("encode notification state %q: %w", st.UserID, err)
	}
	if err := s.state.UpsertItem(ctx, st.UserID, body); err != nil {
		return fmt.Errorf("upsert notification state %q: %w", st.UserID, err)
	}
	return nil
}

// UnreadCount counts the user's notifications created strictly after the
// watermark (the boundary instant itself counts as read). The parameter is
// passed as a "+00:00"-formatted DateTimeOffset string so Cosmos's lexicographic
// comparison lines up with the stored createdAt values.
func (s *CosmosStore) UnreadCount(ctx context.Context, userID string, lastReadAt time.Time) (int, error) {
	const query = "SELECT VALUE COUNT(1) FROM c WHERE c.userId = @userId AND c.createdAt > @lastReadAt"
	count, err := s.notifications.CountItems(ctx, userID, query, map[string]any{
		"@userId":     userID,
		"@lastReadAt": platform.DotNetTime(lastReadAt).String(),
	})
	if err != nil {
		return 0, fmt.Errorf("count unread for %q: %w", userID, err)
	}
	return count, nil
}

// DeleteByUserID removes the user's watermark document for the account-erasure
// cascade (dormant cleanup and DELETE /v1/me). The container holds one document
// per user (id == userId == partition key), so this is a single point delete; a
// 404 is tolerated so an account with no watermark yet is not a cascade failure.
//
// Note: the legacy account-deletion flow did not erase the notification-state
// watermark, leaving an orphaned document after account deletion; this cascade
// deletes it (epic tc-wad3, bead tc-dwcq) for complete GDPR erasure.
func (s *CosmosStore) DeleteByUserID(ctx context.Context, userID string) error {
	if err := s.state.DeleteItem(ctx, userID, userID); err != nil && !platform.IsCosmosNotFound(err) {
		return fmt.Errorf("delete notification state %q: %w", userID, err)
	}
	return nil
}
