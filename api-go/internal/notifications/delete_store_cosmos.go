package notifications

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// DeleteItems is the consumer-side slice of the Notifications container the
// cascade-delete store uses: a single-partition id projection plus a
// single-partition point delete. platform.CosmosContainer satisfies it
// structurally. It is separate from CosmosItems / DigestItems because account
// erasure needs delete, which the read-only projection store does not expose.
type DeleteItems interface {
	QueryItems(ctx context.Context, partitionKey, query string, params map[string]any) ([][]byte, error)
	DeleteItem(ctx context.Context, partitionKey, id string) error
}

// DeleteStore removes a user's notifications for the account-erasure cascade
// (dormant cleanup and DELETE /v1/me).
//
// Partition strategy: the Notifications container is partitioned by /userId, so
// both the id projection and the point deletes target the single user partition
// and never fan out cross-partition.
type DeleteStore struct {
	items DeleteItems
}

// NewDeleteStore returns a cascade-delete store backed by the given accessor.
func NewDeleteStore(items DeleteItems) *DeleteStore {
	return &DeleteStore{items: items}
}

// deleteIDOnlyDocument captures just the id projected by the cascade-delete query
// (SELECT c.id FROM c ...).
type deleteIDOnlyDocument struct {
	ID string `json:"id"`
}

// DeleteAllByUserID removes every notification in the user's partition: it
// queries the partition for the document ids, then point-deletes each. A 404 on
// an individual delete is tolerated so a concurrent delete does not fail the cascade.
func (s *DeleteStore) DeleteAllByUserID(ctx context.Context, userID string) error {
	const query = "SELECT c.id FROM c WHERE c.userId = @userId"
	raws, err := s.items.QueryItems(ctx, userID, query, map[string]any{"@userId": userID})
	if err != nil {
		return fmt.Errorf("query notification ids for %q: %w", userID, err)
	}
	for _, raw := range raws {
		var doc deleteIDOnlyDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("decode notification id for %q: %w", userID, err)
		}
		if err := s.items.DeleteItem(ctx, userID, doc.ID); err != nil && !platform.IsCosmosNotFound(err) {
			return fmt.Errorf("delete notification %q for %q: %w", doc.ID, userID, err)
		}
	}
	return nil
}
