package watchzones

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// ErrNotFound signals that no watch zone exists for the given (user, zone) pair.
// Callers use errors.Is to translate it to a 404, mirroring .NET's
// WatchZoneNotFoundException / null-return paths.
var ErrNotFound = errors.New("watch zone not found")

// CosmosItems is the consumer-side slice of the Cosmos container the store uses:
// a single-partition point read/upsert/delete plus a single-partition query for
// the per-user list. Defining it here keeps azcosmos types out of the store's
// unit tests, which substitute a hand-written fake. platform.CosmosContainer
// satisfies it structurally.
type CosmosItems interface {
	ReadItem(ctx context.Context, partitionKey, id string) ([]byte, error)
	UpsertItem(ctx context.Context, partitionKey string, item []byte) error
	DeleteItem(ctx context.Context, partitionKey, id string) error
	QueryItems(ctx context.Context, partitionKey, query string, params map[string]any) ([][]byte, error)
}

// listByUserQuery lists a user's zones. It is scoped to the userId partition, so
// it never fans out cross-partition — matching .NET's CosmosWatchZoneRepository.
const listByUserQuery = "SELECT * FROM c WHERE c.userId = @userId"

// CosmosStore reads and writes watch zones in the WatchZones container. It holds
// only the consumer-side item interface, so no SDK type leaks past it.
//
// Partition strategy: the WatchZones container is partitioned by /userId; the
// document id equals the zone id. A single-zone operation is a point operation
// keyed on (userId, zoneId); a user's list is one single-partition query.
type CosmosStore struct {
	items CosmosItems
}

// NewCosmosStore returns a store backed by the given Cosmos item accessor.
func NewCosmosStore(items CosmosItems) *CosmosStore {
	return &CosmosStore{items: items}
}

// GetByUserID returns all of the user's zones via a single-partition query.
func (s *CosmosStore) GetByUserID(ctx context.Context, userID string) ([]WatchZone, error) {
	raws, err := s.items.QueryItems(ctx, userID, listByUserQuery, map[string]any{"@userId": userID})
	if err != nil {
		return nil, fmt.Errorf("query watch zones for %q: %w", userID, err)
	}
	zones := make([]WatchZone, 0, len(raws))
	for _, raw := range raws {
		var doc watchZoneDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("decode watch zone for %q: %w", userID, err)
		}
		zone, err := doc.toDomain()
		if err != nil {
			return nil, fmt.Errorf("hydrate watch zone for %q: %w", userID, err)
		}
		zones = append(zones, zone)
	}
	return zones, nil
}

// Get point-reads a single zone. A 404 from Cosmos surfaces as ErrNotFound.
func (s *CosmosStore) Get(ctx context.Context, userID, zoneID string) (WatchZone, error) {
	raw, err := s.items.ReadItem(ctx, userID, zoneID)
	if err != nil {
		if isNotFound(err) {
			return WatchZone{}, ErrNotFound
		}
		return WatchZone{}, fmt.Errorf("read watch zone %q: %w", zoneID, err)
	}
	var doc watchZoneDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return WatchZone{}, fmt.Errorf("decode watch zone %q: %w", zoneID, err)
	}
	zone, err := doc.toDomain()
	if err != nil {
		return WatchZone{}, fmt.Errorf("hydrate watch zone %q: %w", zoneID, err)
	}
	return zone, nil
}

// Save upserts the zone document (partition key == user id, id == zone id).
func (s *CosmosStore) Save(ctx context.Context, z WatchZone) error {
	body, err := json.Marshal(newWatchZoneDocument(z))
	if err != nil {
		return fmt.Errorf("encode watch zone %q: %w", z.ID, err)
	}
	if err := s.items.UpsertItem(ctx, z.UserID, body); err != nil {
		return fmt.Errorf("upsert watch zone %q: %w", z.ID, err)
	}
	return nil
}

// Delete removes a zone. A 404 surfaces as ErrNotFound so the handler can return
// the .NET 404, mirroring WatchZoneNotFoundException. (The azcosmos delete is
// not idempotent — it 404s on a missing id — so no read-first is needed, unlike
// the .NET REST client.)
func (s *CosmosStore) Delete(ctx context.Context, userID, zoneID string) error {
	if err := s.items.DeleteItem(ctx, userID, zoneID); err != nil {
		if isNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("delete watch zone %q: %w", zoneID, err)
	}
	return nil
}

// isNotFound reports whether err is a Cosmos 404 response.
func isNotFound(err error) bool {
	var respErr *azcore.ResponseError
	return errors.As(err, &respErr) && respErr.StatusCode == http.StatusNotFound
}
