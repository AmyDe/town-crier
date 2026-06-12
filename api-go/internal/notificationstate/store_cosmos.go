package notificationstate

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

// CosmosItems is the consumer-side slice of the NotificationState container the
// store uses: point read/upsert keyed on the user id (one document per user,
// id == partition key). platform.CosmosContainer satisfies it structurally.
type CosmosItems interface {
	ReadItem(ctx context.Context, partitionKey, id string) ([]byte, error)
	UpsertItem(ctx context.Context, partitionKey string, item []byte) error
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
// the first-touch signal the handlers branch on, mirroring .NET's null return.
func (s *CosmosStore) Get(ctx context.Context, userID string) (*State, error) {
	raw, err := s.state.ReadItem(ctx, userID, userID)
	if err != nil {
		if isNotFound(err) {
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
// watermark (the boundary instant itself counts as read), mirroring .NET
// GetUnreadCountAsync. The parameter is passed in the .NET DateTimeOffset
// string form so Cosmos's lexicographic string comparison lines up with the
// "+00:00"-formatted createdAt values .NET writes.
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

// isNotFound reports whether err is a Cosmos 404 response.
func isNotFound(err error) bool {
	var respErr *azcore.ResponseError
	return errors.As(err, &respErr) && respErr.StatusCode == http.StatusNotFound
}
