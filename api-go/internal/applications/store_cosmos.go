package applications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// CosmosItems is the consumer-side slice of the Applications container the store
// uses: a single-partition point read and an upsert. platform.CosmosContainer
// satisfies it structurally.
type CosmosItems interface {
	ReadItem(ctx context.Context, partitionKey, id string) ([]byte, error)
	UpsertItem(ctx context.Context, partitionKey string, item []byte) error
}

// CosmosStore reads and writes planning applications in the Applications
// container.
//
// Partition strategy: the container is partitioned by /authorityCode (the AreaID
// as a string); the document id is the PlanIt case reference (Name). A lookup by
// (authorityCode, name) is a ~1 RU point read; an upsert targets the
// authorityCode partition.
type CosmosStore struct {
	items CosmosItems
}

// NewCosmosStore returns a store backed by the given Cosmos item accessor.
func NewCosmosStore(items CosmosItems) *CosmosStore {
	return &CosmosStore{items: items}
}

// Upsert writes the application document into its authorityCode partition.
func (s *CosmosStore) Upsert(ctx context.Context, a PlanningApplication) error {
	body, err := json.Marshal(newApplicationDocument(a))
	if err != nil {
		return fmt.Errorf("encode application %q: %w", a.Name, err)
	}
	if err := s.items.UpsertItem(ctx, strconv.Itoa(a.AreaID), body); err != nil {
		return fmt.Errorf("upsert application %q: %w", a.Name, err)
	}
	return nil
}

// GetByAuthorityAndName point-reads the application identified by (authorityCode,
// name). The boolean reports presence: a missing application is a normal 404 for
// the caller, not an error. There is no PlanIt fallback (GH#395 Invariant 1).
func (s *CosmosStore) GetByAuthorityAndName(ctx context.Context, authorityCode, name string) (PlanningApplication, bool, error) {
	raw, err := s.items.ReadItem(ctx, authorityCode, name)
	if err != nil {
		if isNotFound(err) {
			return PlanningApplication{}, false, nil
		}
		return PlanningApplication{}, false, fmt.Errorf("read application %q/%q: %w", authorityCode, name, err)
	}
	var doc applicationDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return PlanningApplication{}, false, fmt.Errorf("decode application %q/%q: %w", authorityCode, name, err)
	}
	return doc.toDomain(), true, nil
}

// isNotFound reports whether err is a Cosmos 404 response.
func isNotFound(err error) bool {
	var respErr *azcore.ResponseError
	return errors.As(err, &respErr) && respErr.StatusCode == http.StatusNotFound
}
