package offercodes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// ErrNotFound signals that no offer code exists for the given canonical code.
// The redeem handler translates it to the 404 "invalid_code" response.
var ErrNotFound = errors.New("offer code not found")

// cosmosItems is the consumer-side slice of the Cosmos container the store
// uses: single-partition point read/upsert keyed on the canonical code, a
// CAS-aware read+replace for atomic redemption, and a cross-partition scan for
// the GDPR anonymise path (the container is keyed by code, not by redeemer, so
// finding a user's redemptions must fan out across partitions). The
// azcosmos-backed platform.CosmosContainer satisfies it structurally.
type cosmosItems interface {
	ReadItem(ctx context.Context, partitionKey, id string) ([]byte, error)
	UpsertItem(ctx context.Context, partitionKey string, item []byte) error
	ReadItemWithETag(ctx context.Context, partitionKey, id string) (body []byte, etag string, found bool, err error)
	ReplaceItemWithETag(ctx context.Context, partitionKey, id string, item []byte, etag string) (string, error)
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

// RedeemWithCAS atomically redeems the code using a read+etag-conditional
// replace so concurrent redeems of the same code yield at most one success.
//
// Returns:
//   - nil on success (the code is now persisted as redeemed).
//   - ErrNotFound if the code does not exist.
//   - ErrAlreadyRedeemed if the code is already redeemed at read time.
//   - platform.ErrCASPreconditionFailed if a concurrent writer mutated the
//     document between the read and the replace (412 etag mismatch). The caller
//     should re-read and retry, or treat it as code_already_redeemed.
func (s *CosmosStore) RedeemWithCAS(ctx context.Context, canonical, userID string, now time.Time) error {
	raw, etag, found, err := s.items.ReadItemWithETag(ctx, canonical, canonical)
	if err != nil {
		return fmt.Errorf("read offer code %q for CAS redeem: %w", canonical, err)
	}
	if !found {
		return ErrNotFound
	}
	var doc offerCodeDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("decode offer code %q: %w", canonical, err)
	}
	code, err := doc.toDomain()
	if err != nil {
		return fmt.Errorf("parse offer code %q: %w", canonical, err)
	}
	if code.IsRedeemed() {
		return ErrAlreadyRedeemed
	}
	if err := code.Redeem(userID, now); err != nil {
		return fmt.Errorf("redeem offer code %q: %w", canonical, err)
	}
	updated, err := json.Marshal(newOfferCodeDocument(code))
	if err != nil {
		return fmt.Errorf("encode redeemed offer code %q: %w", canonical, err)
	}
	if _, err := s.items.ReplaceItemWithETag(ctx, canonical, canonical, updated, etag); err != nil {
		// ErrCASPreconditionFailed surfaces unwrapped so the handler can detect it
		// with errors.Is and decide whether to retry or map to 409.
		return fmt.Errorf("replace offer code %q: %w", canonical, err)
	}
	return nil
}

// redeemedByUserIDQuery selects every code a user redeemed. The OfferCodes
// container is partitioned by /id == the canonical code, not by redeemer, so
// this is a deliberate cross-partition scan.
const redeemedByUserIDQuery = "SELECT * FROM c WHERE c.redeemedByUserId = @userId"

// RedeemedByUserID returns every code the user redeemed, hydrated to the domain
// model, for the GDPR data export (GET /v1/me/data). It reuses the same
// cross-partition redeemedByUserId scan as the anonymise path (the OfferCodes
// container is partitioned by /id == the canonical code, not by redeemer, so
// finding a user's redemptions must fan out across partitions). The common case
// (the user never redeemed a code) matches nothing and returns an empty, non-nil
// slice. The export sorts the result, so no ORDER BY is needed here.
func (s *CosmosStore) RedeemedByUserID(ctx context.Context, userID string) ([]OfferCode, error) {
	raws, err := s.items.QueryItemsCrossPartition(ctx, redeemedByUserIDQuery, map[string]any{"@userId": userID})
	if err != nil {
		return nil, fmt.Errorf("query redemptions for user %q: %w", userID, err)
	}
	codes := make([]OfferCode, 0, len(raws))
	for _, raw := range raws {
		var doc offerCodeDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("decode redeemed offer code for user %q: %w", userID, err)
		}
		code, err := doc.toDomain()
		if err != nil {
			return nil, fmt.Errorf("hydrate redeemed offer code for user %q: %w", userID, err)
		}
		codes = append(codes, code)
	}
	return codes, nil
}

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
