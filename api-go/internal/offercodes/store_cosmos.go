package offercodes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// ErrNotFound signals that no offer code exists for the given canonical code.
// The redeem handler translates it to the 404 "invalid_code" response.
var ErrNotFound = errors.New("offer code not found")

// cosmosItems is the consumer-side slice of the Cosmos container the store
// uses: single-partition point read/upsert keyed on the canonical code. The
// azcosmos-backed platform.CosmosContainer satisfies it structurally.
type cosmosItems interface {
	ReadItem(ctx context.Context, partitionKey, id string) ([]byte, error)
	UpsertItem(ctx context.Context, partitionKey string, item []byte) error
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
		if isNotFound(err) {
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

func isNotFound(err error) bool {
	var respErr *azcore.ResponseError
	return errors.As(err, &respErr) && respErr.StatusCode == http.StatusNotFound
}
