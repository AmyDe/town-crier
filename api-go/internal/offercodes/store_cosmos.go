package offercodes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// ErrNotFound signals that no offer code exists for the given canonical code.
// The redeem handler translates it to the 404 "invalid_code" response.
var ErrNotFound = errors.New("offer code not found")

// cosmosItems is the consumer-side slice of the Cosmos container the store
// uses: single-partition point read/upsert keyed on the canonical code, plus a
// cross-partition scan for the GDPR anonymise path (the container is keyed by
// code, not by redeemer, so finding a user's redemptions must fan out across
// partitions). The azcosmos-backed platform.CosmosContainer satisfies it
// structurally.
type cosmosItems interface {
	ReadItem(ctx context.Context, partitionKey, id string) ([]byte, error)
	UpsertItem(ctx context.Context, partitionKey string, item []byte) error
	QueryItemsCrossPartition(ctx context.Context, query string, params map[string]any) ([][]byte, error)
}

// CosmosStore reads and writes offer codes in the OfferCodes container.
//
// Partition strategy: the OfferCodes container is partitioned by /id == the
// canonical code, so every operation is a single-partition point operation.
type CosmosStore struct {
	items cosmosItems
}

// NewCosmosStore returns a store backed by the given Cosmos item accessor.
func NewCosmosStore(items cosmosItems) *CosmosStore { return &CosmosStore{items: items} }

// Get point-reads the code; a 404 from Cosmos surfaces as ErrNotFound.
func (s *CosmosStore) Get(ctx context.Context, canonical string) (OfferCode, error) {
	raw, err := s.items.ReadItem(ctx, canonical, canonical)
	if err != nil {
		if platform.IsCosmosNotFound(err) {
			return OfferCode{}, ErrNotFound
		}
		return OfferCode{}, fmt.Errorf("read offer code %q: %w", canonical, err)
	}
	var doc offerCodeDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return OfferCode{}, fmt.Errorf("decode offer code %q: %w", canonical, err)
	}
	return doc.toDomain()
}

// Save upserts the code document (id == code == partition key). The .NET
// repository's CreateAsync is a best-effort upsert too, so Save covers both.
func (s *CosmosStore) Save(ctx context.Context, c OfferCode) error {
	body, err := json.Marshal(newOfferCodeDocument(c))
	if err != nil {
		return fmt.Errorf("encode offer code %q: %w", c.Code, err)
	}
	if err := s.items.UpsertItem(ctx, c.Code, body); err != nil {
		return fmt.Errorf("upsert offer code %q: %w", c.Code, err)
	}
	return nil
}

// redeemedByUserIDQuery selects every code a user redeemed. The OfferCodes
// container is partitioned by /id == the canonical code, not by redeemer, so
// this is a deliberate cross-partition scan.
const redeemedByUserIDQuery = "SELECT * FROM c WHERE c.redeemedByUserId = @userId"

// AnonymiseRedemptionsByUserID scrubs the redeemer back-reference
// (redeemedByUserId + redeemedAt) from every code the user redeemed, for UK
// GDPR Art. 17 account erasure. It does NOT delete the code document (the code
// belongs to the admin campaign) nor clear the consumed tombstone (the code can
// only ever be redeemed once and the audit that it WAS redeemed must survive),
// so an anonymised code stays redeemed and cannot be re-redeemed. Both erasure
// entry points — DELETE /v1/me and the dormant-cleanup worker — call it.
//
// The common case (the user never redeemed a code) matches nothing and is a
// no-op success. Each upsert rewrites the document in its own partition (id ==
// code), so the operation is idempotent.
func (s *CosmosStore) AnonymiseRedemptionsByUserID(ctx context.Context, userID string) error {
	raws, err := s.items.QueryItemsCrossPartition(ctx, redeemedByUserIDQuery, map[string]any{"@userId": userID})
	if err != nil {
		return fmt.Errorf("query redemptions for user %q: %w", userID, err)
	}
	for _, raw := range raws {
		var doc offerCodeDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("decode redeemed offer code for user %q: %w", userID, err)
		}
		doc.Redeemed = true // retain the consumed state so the code can't be re-redeemed
		doc.RedeemedByUserID = nil
		doc.RedeemedAt = nil
		body, err := json.Marshal(doc)
		if err != nil {
			return fmt.Errorf("encode anonymised offer code %q: %w", doc.Code, err)
		}
		if err := s.items.UpsertItem(ctx, doc.Code, body); err != nil {
			return fmt.Errorf("upsert anonymised offer code %q: %w", doc.Code, err)
		}
	}
	return nil
}
