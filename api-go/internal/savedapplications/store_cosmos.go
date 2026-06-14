package savedapplications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// CosmosItems is the consumer-side slice of the SavedApplications container the
// store uses: point read/upsert/delete, a single-partition list query, and a
// cross-partition scan for the poll-path decision fan-out (who saved a given
// application). platform.CosmosContainer satisfies it structurally.
type CosmosItems interface {
	ReadItem(ctx context.Context, partitionKey, id string) ([]byte, error)
	UpsertItem(ctx context.Context, partitionKey string, item []byte) error
	DeleteItem(ctx context.Context, partitionKey, id string) error
	QueryItems(ctx context.Context, partitionKey, query string, params map[string]any) ([][]byte, error)
	QueryItemsCrossPartition(ctx context.Context, query string, params map[string]any) ([][]byte, error)
}

// listByUserQuery lists a user's saved applications. Scoped to the userId
// partition, so it never fans out cross-partition.
const listByUserQuery = "SELECT * FROM c WHERE c.userId = @userId"

// CosmosStore reads and writes saved applications in the SavedApplications
// container.
//
// Partition strategy: partitioned by /userId; the document id is
// "{userId}:{applicationUid}". A save/exists/delete is a point operation; a
// user's list is one single-partition query.
type CosmosStore struct {
	items CosmosItems
}

// NewCosmosStore returns a store backed by the given Cosmos item accessor.
func NewCosmosStore(items CosmosItems) *CosmosStore {
	return &CosmosStore{items: items}
}

// Save upserts the saved-application document into the user's partition.
func (s *CosmosStore) Save(ctx context.Context, sa SavedApplication) error {
	body, err := json.Marshal(newSavedApplicationDocument(sa))
	if err != nil {
		return fmt.Errorf("encode saved application %q: %w", sa.ApplicationUID, err)
	}
	if err := s.items.UpsertItem(ctx, sa.UserID, body); err != nil {
		return fmt.Errorf("upsert saved application %q: %w", sa.ApplicationUID, err)
	}
	return nil
}

// Exists reports whether the user has saved the application with the given
// (canonical) uid.
func (s *CosmosStore) Exists(ctx context.Context, userID, applicationUID string) (bool, error) {
	_, err := s.items.ReadItem(ctx, userID, makeID(userID, applicationUID))
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("read saved application %q: %w", applicationUID, err)
	}
	return true, nil
}

// Delete removes the saved-application document. A missing document is not an
// error: the .NET REST delete is idempotent and the DELETE endpoint always
// returns 204, so a 404 from Cosmos is swallowed.
func (s *CosmosStore) Delete(ctx context.Context, userID, applicationUID string) error {
	if err := s.items.DeleteItem(ctx, userID, makeID(userID, applicationUID)); err != nil && !isNotFound(err) {
		return fmt.Errorf("delete saved application %q: %w", applicationUID, err)
	}
	return nil
}

// GetByUserID returns the user's saved applications via a single-partition query.
func (s *CosmosStore) GetByUserID(ctx context.Context, userID string) ([]SavedApplication, error) {
	raws, err := s.items.QueryItems(ctx, userID, listByUserQuery, map[string]any{"@userId": userID})
	if err != nil {
		return nil, fmt.Errorf("query saved applications for %q: %w", userID, err)
	}
	saved := make([]SavedApplication, 0, len(raws))
	for _, raw := range raws {
		var doc savedApplicationDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("decode saved application for %q: %w", userID, err)
		}
		saved = append(saved, doc.toDomain())
	}
	return saved, nil
}

// userIDsForApplicationQuery projects the distinct user ids that have saved the
// (applicationUid, authorityId), across all partitions. The authority predicate
// is load-bearing: PlanIt uids collide across councils (tc-th98 / GH#384), so a
// uid-only match would falsely fan out a decision to bookmark holders in another
// authority. Mirrors .NET GetUserIdsForApplicationCrossPartitionAsync. Legacy
// rows persisted before the authorityId column was added fall through the filter
// naturally — they won't match until backfilled.
const userIDsForApplicationQuery = "SELECT VALUE c.userId FROM c " +
	"WHERE c.applicationUid = @applicationUid AND c.authorityId = @authorityId"

// UserIDsForApplication returns every user id that has saved the given
// (applicationUid, authorityId), via a cross-partition scan. It backs the
// poll-path decision-event fan-out to bookmark holders (epic tc-wad3, bead
// tc-uc2p). This is a deliberate cross-partition scan — saved-app docs are
// partitioned by userId, but a polled decision needs every user who saved it.
func (s *CosmosStore) UserIDsForApplication(ctx context.Context, applicationUID string, authorityID int) ([]string, error) {
	raws, err := s.items.QueryItemsCrossPartition(ctx, userIDsForApplicationQuery, map[string]any{
		"@applicationUid": applicationUID,
		"@authorityId":    authorityID,
	})
	if err != nil {
		return nil, fmt.Errorf("query user ids for saved application %q: %w", applicationUID, err)
	}
	userIDs := make([]string, 0, len(raws))
	for _, raw := range raws {
		var id string
		if err := json.Unmarshal(raw, &id); err != nil {
			return nil, fmt.Errorf("decode saved-application user id: %w", err)
		}
		userIDs = append(userIDs, id)
	}
	return userIDs, nil
}

// idOnlyDocument captures just the id projected by the cascade-delete query
// (SELECT c.id FROM c ...), so the cascade need not hydrate full documents.
type idOnlyDocument struct {
	ID string `json:"id"`
}

// DeleteAllByUserID removes every saved application in the user's partition: it
// queries the partition for the document ids, then point-deletes each. Used by
// the account-deletion cascade (dormant cleanup and DELETE /v1/me), mirroring
// .NET CosmosSavedApplicationRepository.DeleteAllByUserIdAsync. All operations
// are single-partition.
func (s *CosmosStore) DeleteAllByUserID(ctx context.Context, userID string) error {
	raws, err := s.items.QueryItems(ctx, userID, "SELECT c.id FROM c WHERE c.userId = @userId", map[string]any{"@userId": userID})
	if err != nil {
		return fmt.Errorf("query saved application ids for %q: %w", userID, err)
	}
	for _, raw := range raws {
		var doc idOnlyDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("decode saved application id for %q: %w", userID, err)
		}
		if err := s.items.DeleteItem(ctx, userID, doc.ID); err != nil && !isNotFound(err) {
			return fmt.Errorf("delete saved application %q for %q: %w", doc.ID, userID, err)
		}
	}
	return nil
}

// isNotFound reports whether err is a Cosmos 404 response.
func isNotFound(err error) bool {
	var respErr *azcore.ResponseError
	return errors.As(err, &respErr) && respErr.StatusCode == http.StatusNotFound
}
