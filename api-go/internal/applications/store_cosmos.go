package applications

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// CosmosItems is the consumer-side slice of the Applications container the store
// uses: a single-partition point read, an upsert, and a single-partition
// spatial query for the nearby lookup. platform.CosmosContainer satisfies it
// structurally.
type CosmosItems interface {
	ReadItem(ctx context.Context, partitionKey, id string) ([]byte, error)
	UpsertItem(ctx context.Context, partitionKey string, item []byte) error
	QueryItems(ctx context.Context, partitionKey, query string, params map[string]any) ([][]byte, error)
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
		if platform.IsCosmosNotFound(err) {
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

// GetByUID looks up an application by its raw PlanIt uid within the authorityCode
// partition, via a single-partition query on the uid field. Used by the
// saved-application lazy snapshot backfill, where a legacy row holds the bare
// uid and the authority is known.
// The boolean reports presence; a miss is normal (the master record may be gone).
func (s *CosmosStore) GetByUID(ctx context.Context, uid, authorityCode string) (PlanningApplication, bool, error) {
	const query = "SELECT * FROM c WHERE c.uid = @uid"
	raws, err := s.items.QueryItems(ctx, authorityCode, query, map[string]any{"@uid": uid})
	if err != nil {
		return PlanningApplication{}, false, fmt.Errorf("query application uid %q in %q: %w", uid, authorityCode, err)
	}
	if len(raws) == 0 {
		return PlanningApplication{}, false, nil
	}
	var doc applicationDocument
	if err := json.Unmarshal(raws[0], &doc); err != nil {
		return PlanningApplication{}, false, fmt.Errorf("decode application uid %q: %w", uid, err)
	}
	return doc.toDomain(), true, nil
}

// findNearbyQuery is the single-partition ST_DISTANCE spatial query for nearby
// applications. Coordinates and radius are bound as named parameters (mirroring
// findZonesContainingQuery in the watchzones package) — no float literals are
// concatenated into the query text.
const findNearbyQuery = "SELECT * FROM c WHERE ST_DISTANCE(c.location, " +
	`{"type": "Point", "coordinates": [@longitude, @latitude]}) <= @radiusMetres`

// FindNearby returns every application within radiusMetres of (latitude,
// longitude) inside the authorityCode partition, via a single-partition
// ST_DISTANCE spatial query against the GeoJSON location. Coordinates and
// radius are bound as named parameters (not string-concatenated) to eliminate
// float-formatting edge cases and to mirror the parameterized style of the
// sibling watchzones.FindZonesContaining query. The query is scoped to the
// authorityCode logical partition, so it never fans out cross-partition.
func (s *CosmosStore) FindNearby(ctx context.Context, authorityCode string, latitude, longitude, radiusMetres float64) ([]PlanningApplication, error) {
	params := map[string]any{
		"@latitude":     latitude,
		"@longitude":    longitude,
		"@radiusMetres": radiusMetres,
	}
	raws, err := s.items.QueryItems(ctx, authorityCode, findNearbyQuery, params)
	if err != nil {
		return nil, fmt.Errorf("find applications near %q: %w", authorityCode, err)
	}
	apps := make([]PlanningApplication, 0, len(raws))
	for _, raw := range raws {
		var doc applicationDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("decode nearby application in %q: %w", authorityCode, err)
		}
		apps = append(apps, doc.toDomain())
	}
	return apps, nil
}
