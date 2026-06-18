package profiles

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// ErrNotFound signals that no profile exists for the given user id. Callers use
// errors.Is to translate it to a 404 response, mirroring .NET's null-return /
// UserProfileNotFoundException paths.
var ErrNotFound = errors.New("user profile not found")

// CosmosItems is the consumer-side slice of the Cosmos container the store uses:
// single-partition point read/upsert/delete keyed on the user id. Defining it
// here (not in the SDK adapter) keeps azcosmos types out of the store's unit
// tests, which substitute a hand-written fake. The azcosmos-backed
// implementation (platform.CosmosContainer) satisfies it structurally — the
// methods are exported so a type in another package can implement them.
type CosmosItems interface {
	ReadItem(ctx context.Context, partitionKey, id string) ([]byte, error)
	UpsertItem(ctx context.Context, partitionKey string, item []byte) error
	DeleteItem(ctx context.Context, partitionKey, id string) error
}

// cosmosItemsCAS extends CosmosItems with the etag-conditional operations
// needed for the watch-zone quota CAS loop. platform.CosmosContainer satisfies
// it structurally — the methods are unexported here because only this package
// uses the extended interface.
type cosmosItemsCAS interface {
	CosmosItems
	ReadItemWithETag(ctx context.Context, partitionKey, id string) (body []byte, etag string, found bool, err error)
	ReplaceItemWithETag(ctx context.Context, partitionKey, id string, item []byte, etag string) (string, error)
}

// CosmosStore reads and writes user profiles in the Users container. It holds
// only the consumer-side item interface, so no SDK type leaks past it.
//
// Partition strategy: the Users container is partitioned by /id == the Auth0
// user id, so every operation is a single-partition point operation. No
// cross-partition query is needed for the /v1/me lifecycle.
type CosmosStore struct {
	items    CosmosItems
	casItems cosmosItemsCAS // nil when the container doesn't support CAS
}

// NewCosmosStore returns a store backed by the given Cosmos item accessor. When
// the container also satisfies cosmosItemsCAS (i.e. platform.CosmosContainer),
// the store automatically gains GetWithETag / UpdateZoneCountWithCAS.
func NewCosmosStore(items CosmosItems) *CosmosStore {
	s := &CosmosStore{items: items}
	if cas, ok := items.(cosmosItemsCAS); ok {
		s.casItems = cas
	}
	return s
}

// Get point-reads the profile for userID. A 404 from Cosmos surfaces as
// ErrNotFound; any other failure is wrapped and returned.
func (s *CosmosStore) Get(ctx context.Context, userID string) (*UserProfile, error) {
	raw, err := s.items.ReadItem(ctx, userID, userID)
	if err != nil {
		if platform.IsCosmosNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read profile %q: %w", userID, err)
	}
	var doc profileDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decode profile %q: %w", userID, err)
	}
	profile, err := doc.toDomain()
	if err != nil {
		return nil, fmt.Errorf("hydrate profile %q: %w", userID, err)
	}
	return profile, nil
}

// Save upserts the profile document (id == user id == partition key).
func (s *CosmosStore) Save(ctx context.Context, p *UserProfile) error {
	body, err := json.Marshal(newProfileDocument(p))
	if err != nil {
		return fmt.Errorf("encode profile %q: %w", p.UserID, err)
	}
	if err := s.items.UpsertItem(ctx, p.UserID, body); err != nil {
		return fmt.Errorf("upsert profile %q: %w", p.UserID, err)
	}
	return nil
}

// Delete removes the profile document. A 404 surfaces as ErrNotFound so the
// caller can decide whether that is an error (it is for DELETE /v1/me, which
// reads first) or tolerable.
func (s *CosmosStore) Delete(ctx context.Context, userID string) error {
	if err := s.items.DeleteItem(ctx, userID, userID); err != nil {
		if platform.IsCosmosNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("delete profile %q: %w", userID, err)
	}
	return nil
}

// GetWithETag reads the profile and its current etag for a CAS operation. It
// returns (nil, "", nil) when the profile does not exist. An error is returned
// on any store failure. This method requires the store to have been constructed
// with a container that supports CAS operations.
func (s *CosmosStore) GetWithETag(ctx context.Context, userID string) (*UserProfile, string, error) {
	if s.casItems == nil {
		return nil, "", fmt.Errorf("profile store: CAS not available (container does not support ReadItemWithETag)")
	}
	raw, etag, found, err := s.casItems.ReadItemWithETag(ctx, userID, userID)
	if err != nil {
		return nil, "", fmt.Errorf("read profile %q with etag: %w", userID, err)
	}
	if !found {
		return nil, "", nil
	}
	var doc profileDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, "", fmt.Errorf("decode profile %q: %w", userID, err)
	}
	profile, err := doc.toDomain()
	if err != nil {
		return nil, "", fmt.Errorf("hydrate profile %q: %w", userID, err)
	}
	return profile, etag, nil
}

// UpdateZoneCountWithCAS replaces the profile document only if its stored etag
// matches, persisting the updated WatchZoneCount. A 412 (etag mismatch) from
// Cosmos surfaces as platform.ErrCASPreconditionFailed so the caller can
// re-read and retry. Any other store failure is wrapped and returned.
func (s *CosmosStore) UpdateZoneCountWithCAS(ctx context.Context, userID string, p *UserProfile, etag string) error {
	if s.casItems == nil {
		return fmt.Errorf("profile store: CAS not available (container does not support ReplaceItemWithETag)")
	}
	body, err := json.Marshal(newProfileDocument(p))
	if err != nil {
		return fmt.Errorf("encode profile %q for CAS replace: %w", userID, err)
	}
	if _, err := s.casItems.ReplaceItemWithETag(ctx, userID, userID, body, etag); err != nil {
		// ErrCASPreconditionFailed surfaces unwrapped via %w so the caller can
		// detect it with errors.Is and decide whether to retry or return 403.
		return fmt.Errorf("CAS replace profile %q: %w", userID, err)
	}
	return nil
}
