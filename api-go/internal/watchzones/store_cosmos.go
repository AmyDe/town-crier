package watchzones

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// ErrNotFound signals that no watch zone exists for the given (user, zone) pair.
// Callers use errors.Is to translate it to a 404.
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
	// QueryItemsCrossPartition backs the polling authority provider's
	// authority-id projection (deduped client-side, since azcosmos cannot serve a
	// cross-partition DISTINCT — gateway 400, tc-b7cm) and the spatial fan-out.
	QueryItemsCrossPartition(ctx context.Context, query string, params map[string]any) ([][]byte, error)
}

// listByUserQuery lists a user's zones. It is scoped to the userId partition, so
// it never fans out cross-partition. The ORDER BY c.id makes the result
// deterministic: without it Cosmos returns the zones in an arbitrary order per
// request, which flaked the GDPR export's zonePreferences array order (tc-zgnt).
// The document id equals the zone id, so this orders by zone id.
const listByUserQuery = "SELECT * FROM c WHERE c.userId = @userId ORDER BY c.id"

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
		if platform.IsCosmosNotFound(err) {
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
// a 404. (The azcosmos delete is not idempotent — it 404s on a missing id —
// so no read-first is needed.)
func (s *CosmosStore) Delete(ctx context.Context, userID, zoneID string) error {
	if err := s.items.DeleteItem(ctx, userID, zoneID); err != nil {
		if platform.IsCosmosNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("delete watch zone %q: %w", zoneID, err)
	}
	return nil
}

// idOnlyDocument captures just the id projected by the cascade-delete query
// (SELECT c.id FROM c ...), so the cascade need not hydrate full documents.
type idOnlyDocument struct {
	ID string `json:"id"`
}

// DeleteAllByUserID removes every watch zone in the user's partition: it queries
// the partition for the document ids, then point-deletes each. Used by the
// account-deletion cascade (dormant cleanup and DELETE /v1/me). The scan and the
// deletes are all single-partition, so they never fan out cross-partition.
func (s *CosmosStore) DeleteAllByUserID(ctx context.Context, userID string) error {
	raws, err := s.items.QueryItems(ctx, userID, "SELECT c.id FROM c WHERE c.userId = @userId", map[string]any{"@userId": userID})
	if err != nil {
		return fmt.Errorf("query watch zone ids for %q: %w", userID, err)
	}
	for _, raw := range raws {
		var doc idOnlyDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("decode watch zone id for %q: %w", userID, err)
		}
		if err := s.items.DeleteItem(ctx, userID, doc.ID); err != nil && !platform.IsCosmosNotFound(err) {
			return fmt.Errorf("delete watch zone %q for %q: %w", doc.ID, userID, err)
		}
	}
	return nil
}

// DistinctAuthorityIDs returns the distinct authority ids across every user's
// watch zones, via a cross-partition projection with client-side dedup (azcosmos
// cannot serve a cross-partition DISTINCT — the gateway returns 400 "can not be
// directly served by the gateway"; tc-b7cm). The VALUE projection returns bare
// JSON integers, one per zone row, so the same authority id repeats once per
// zone in it and is de-duplicated here in first-seen order. It backs the polling
// watch-zone active-authority provider (poll-sb cycle).
func (s *CosmosStore) DistinctAuthorityIDs(ctx context.Context) ([]int, error) {
	raws, err := s.items.QueryItemsCrossPartition(ctx, "SELECT VALUE c.authorityId FROM c", nil)
	if err != nil {
		return nil, fmt.Errorf("query distinct authority ids: %w", err)
	}
	ids := make([]int, 0, len(raws))
	seen := make(map[int]struct{}, len(raws))
	for _, raw := range raws {
		var id int
		if err := json.Unmarshal(raw, &id); err != nil {
			return nil, fmt.Errorf("decode authority id: %w", err)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

// findZonesContainingQuery selects every watch zone scoped to the candidate
// application's authority whose circle (centre + radius) contains the candidate
// point, across all partitions. The authority equality predicate is evaluated by
// Cosmos's default range index, pruning rows before the ST_DISTANCE test runs —
// a WatchZone is authority-scoped (see zone.go), so a zone only ever matches
// applications in its own authority, mirroring the saved-bookmark path's
// app.AreaID scoping (notifydispatch/decision.go). The candidate authority and
// point are bound as parameters.
//
// The distance test is a single index-served clause: ST_DISTANCE(c.location,
// @point) <= c.radiusMetres, served by the spatial index on the persisted GeoJSON
// /location path (tc-quqe), so Cosmos fully uses the index rather than a full
// scan. The legacy NOT IS_DEFINED(c.location) fallback — which distanced against
// an inline [c.longitude, c.latitude] point for zones written before the location
// backfill — was removed (tc-ltlw / Phase 2d) once both environments were fully
// backfilled (tc-xj48) and the write path (#618) guaranteed every new zone
// carries c.location, so no zone can lack it and the fallback was dead weight.
//
// All coordinates are GeoJSON order: [longitude, latitude], not [lat, lng].
//
// The projection is deliberate, not SELECT *: only the columns this hot path
// needs are hydrated. id/userId/createdAt are consumed by the callers;
// name/radiusMetres/authorityId are required by the NewWatchZone constructor so
// the hydrated zone stays valid; pushEnabled/emailInstantEnabled are projected
// (not dropped) because they are nullable *bool that coalesce to true when
// absent, so omitting them would silently re-enable a user's disabled
// notifications if a future caller read them. latitude/longitude are omitted from
// the projection — no caller reads zone coordinates after the match, and the
// distance test now reads only the indexed c.location, not the raw columns.
const findZonesContainingQuery = "SELECT c.id, c.userId, c.name, c.radiusMetres, c.authorityId, c.createdAt, c.pushEnabled, c.emailInstantEnabled " +
	"FROM c WHERE c.authorityId = @authorityId AND " +
	"ST_DISTANCE(c.location, {'type': 'Point', 'coordinates': [@longitude, @latitude]}) <= c.radiusMetres"

// FindZonesContaining returns every watch zone (across all users) scoped to the
// given authority whose circle contains the point (latitude, longitude), via a
// cross-partition query that pre-filters on the authority (index-served) before
// the ST_DISTANCE test against each zone's centre and radius. It backs the
// poll-path notification fan-out and decision-event dispatch (epic tc-wad3, bead
// tc-uc2p); authorityID is the polled application's authority (app.AreaID). This
// is a deliberate cross-partition scan — a polled application's coordinates must
// be matched against every user's zones in that authority.
func (s *CosmosStore) FindZonesContaining(ctx context.Context, authorityID int, latitude, longitude float64) ([]WatchZone, error) {
	raws, err := s.items.QueryItemsCrossPartition(ctx, findZonesContainingQuery, map[string]any{
		"@authorityId": authorityID,
		"@latitude":    latitude,
		"@longitude":   longitude,
	})
	if err != nil {
		return nil, fmt.Errorf("find zones containing point: %w", err)
	}
	zones := make([]WatchZone, 0, len(raws))
	for _, raw := range raws {
		var doc watchZoneDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("decode matched watch zone: %w", err)
		}
		zone, err := doc.toDomain()
		if err != nil {
			return nil, fmt.Errorf("hydrate matched watch zone: %w", err)
		}
		zones = append(zones, zone)
	}
	return zones, nil
}
