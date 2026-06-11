package profiles

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// ErrNotFound signals that no profile exists for the given user id. Callers use
// errors.Is to translate it to a 404 response, mirroring .NET's null-return /
// UserProfileNotFoundException paths.
var ErrNotFound = errors.New("user profile not found")

// cosmosItems is the consumer-side slice of the Cosmos container the store uses:
// single-partition point read/upsert/delete keyed on the user id. Defining it
// here (not in the SDK adapter) keeps azcosmos types out of the store's unit
// tests, which substitute a hand-written fake. The azcosmos-backed
// implementation lives in internal/platform.
type cosmosItems interface {
	readItem(ctx context.Context, partitionKey, id string) ([]byte, error)
	upsertItem(ctx context.Context, partitionKey string, item []byte) error
	deleteItem(ctx context.Context, partitionKey, id string) error
}

// CosmosStore reads and writes user profiles in the Users container. It holds
// only the consumer-side item interface, so no SDK type leaks past it.
//
// Partition strategy: the Users container is partitioned by /id == the Auth0
// user id, so every operation is a single-partition point operation. No
// cross-partition query is needed for the /v1/me lifecycle.
type CosmosStore struct {
	items cosmosItems
}

// NewCosmosStore returns a store backed by the given Cosmos item accessor.
func NewCosmosStore(items cosmosItems) *CosmosStore {
	return &CosmosStore{items: items}
}

// Get point-reads the profile for userID. A 404 from Cosmos surfaces as
// ErrNotFound; any other failure is wrapped and returned.
func (s *CosmosStore) Get(ctx context.Context, userID string) (*UserProfile, error) {
	raw, err := s.items.readItem(ctx, userID, userID)
	if err != nil {
		if isNotFound(err) {
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
	if err := s.items.upsertItem(ctx, p.UserID, body); err != nil {
		return fmt.Errorf("upsert profile %q: %w", p.UserID, err)
	}
	return nil
}

// Delete removes the profile document. A 404 surfaces as ErrNotFound so the
// caller can decide whether that is an error (it is for DELETE /v1/me, which
// reads first) or tolerable.
func (s *CosmosStore) Delete(ctx context.Context, userID string) error {
	if err := s.items.deleteItem(ctx, userID, userID); err != nil {
		if isNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("delete profile %q: %w", userID, err)
	}
	return nil
}

// isNotFound reports whether err is a Cosmos 404 response.
func isNotFound(err error) bool {
	var respErr *azcore.ResponseError
	return errors.As(err, &respErr) && respErr.StatusCode == http.StatusNotFound
}
