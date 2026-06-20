package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// CosmosItems is the consumer-side slice of the Notifications container the store
// uses: a single-partition parametrised query. platform.CosmosContainer
// satisfies it structurally.
type CosmosItems interface {
	QueryItems(ctx context.Context, partitionKey, query string, params map[string]any) ([][]byte, error)
}

// latestUnreadQuery selects a user's unread notifications for a set of
// application uids, newest first. ARRAY_CONTAINS binds the uid set as one
// parameter; the query is scoped to the userId partition so it never fans out
// cross-partition.
const latestUnreadQuery = "SELECT * FROM c " +
	"WHERE c.userId = @userId AND ARRAY_CONTAINS(@uids, c.applicationUid) " +
	"AND c.createdAt > @lastReadAt " +
	"ORDER BY c.createdAt DESC"

// CosmosStore reads the Notifications container.
//
// Partition strategy: the container is partitioned by /userId; this read targets
// a single user's partition. SDK types never leak past the store.
type CosmosStore struct {
	items CosmosItems
}

// NewCosmosStore returns a store backed by the given Cosmos item accessor.
func NewCosmosStore(items CosmosItems) *CosmosStore {
	return &CosmosStore{items: items}
}

// GetLatestUnreadByApplications returns, for each application uid that has at
// least one notification created strictly after lastReadAt, the latest such
// notification — in a single single-partition round trip rather than a per-uid
// N+1 loop. An empty uid set returns an empty map without issuing a query.
// lastReadAt is passed as a "+00:00"-formatted DateTimeOffset string so
// Cosmos's lexicographic comparison lines up with the stored timestamps.
func (s *CosmosStore) GetLatestUnreadByApplications(ctx context.Context, userID string, applicationUIDs []string, lastReadAt time.Time) (map[string]LatestUnread, error) {
	if len(applicationUIDs) == 0 {
		return map[string]LatestUnread{}, nil
	}
	raws, err := s.items.QueryItems(ctx, userID, latestUnreadQuery, map[string]any{
		"@userId":     userID,
		"@uids":       applicationUIDs,
		"@lastReadAt": platform.DotNetTime(lastReadAt).String(),
	})
	if err != nil {
		return nil, fmt.Errorf("query latest unread for %q: %w", userID, err)
	}
	// Rows arrive newest-first, so the first row seen per uid is the latest; keep
	// it and skip later (older) rows for the same uid.
	latest := make(map[string]LatestUnread, len(raws))
	for _, raw := range raws {
		var doc notificationDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("decode notification for %q: %w", userID, err)
		}
		lu := doc.toLatestUnread()
		if _, seen := latest[lu.ApplicationUID]; !seen {
			latest[lu.ApplicationUID] = lu
		}
	}
	return latest, nil
}
