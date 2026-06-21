package applications

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// CosmosItems is the consumer-side slice of the Applications container the store
// uses: a single-partition point read, an upsert, and single-partition queries.
// QueryItems carries the tight 1.5s OLTP per-attempt budget for user-facing
// reads; QueryItemsLongRead carries a longer per-attempt budget for the
// latency-tolerant build-time SEO reads over a LARGE authority partition
// (tc-9tov). platform.CosmosContainer satisfies it structurally.
type CosmosItems interface {
	ReadItem(ctx context.Context, partitionKey, id string) ([]byte, error)
	UpsertItem(ctx context.Context, partitionKey string, item []byte) error
	QueryItems(ctx context.Context, partitionKey, query string, params map[string]any) ([][]byte, error)
	QueryItemsLongRead(ctx context.Context, partitionKey, query string, params map[string]any) ([][]byte, error)
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

// recentByAuthorityQuery is the bounded, index-backed top-N query for the most
// recently active applications in a single authorityCode partition. It rides the
// existing (/authorityCode ASC, /lastDifferent DESC) composite index, so the RU
// cost is bounded by the @cap top-N (an index seek), never the partition size —
// even for authorities holding tens of thousands of documents. Ordering is by
// lastDifferent (most recently active) DESC, NOT startDate: startDate is excluded
// from the indexing policy, so ordering by it would force a full-partition scan.
const recentByAuthorityQuery = "SELECT TOP @cap * FROM c ORDER BY c.lastDifferent DESC"

// RecentByAuthority returns up to cap most-recently-active applications in the
// authorityCode partition, ordered by lastDifferent DESC. It is the read behind
// the build-time SEO endpoint: a single bounded single-partition query, never a
// cross-partition fan-out and never an unbounded scan. There is no PlanIt
// fallback (GH#395 Invariant 1) — it reads only from Cosmos.
func (s *CosmosStore) RecentByAuthority(ctx context.Context, authorityCode string, cap int) ([]PlanningApplication, error) {
	params := map[string]any{"@cap": cap}
	// Latency-tolerant build-time read: a LARGE authority partition legitimately
	// exceeds the 1.5s OLTP budget, so use the longer per-attempt budget (tc-9tov).
	raws, err := s.items.QueryItemsLongRead(ctx, authorityCode, recentByAuthorityQuery, params)
	if err != nil {
		return nil, fmt.Errorf("recent applications for authority %q: %w", authorityCode, err)
	}
	apps := make([]PlanningApplication, 0, len(raws))
	for _, raw := range raws {
		var doc applicationDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("decode recent application in %q: %w", authorityCode, err)
		}
		apps = append(apps, doc.toDomain())
	}
	return apps, nil
}

// breakdownByAuthorityQuery is the exact, index-served per-appState distribution
// over a single authorityCode partition. The GROUP BY is served by the appState
// index, so it hydrates no documents and stays cheap (≈3 RU flat, regardless of
// partition size), and the buckets sum to the EXACT partition total — unlike the
// TOP-@cap bounded read, which saturates at the cap. Each row is a projection
// {"appState": "...", "count": N}; Cosmos OMITS the appState property entirely
// when the value is undefined, so a missing-appState bucket arrives without that
// key (decoded as a nil *string), distinct from an explicit JSON null.
const breakdownByAuthorityQuery = "SELECT c.appState, COUNT(1) AS count FROM c GROUP BY c.appState"

// BreakdownByAuthority returns the per-appState distribution over the WHOLE
// authorityCode partition, ordered count DESC then appState ASC with nil last
// (sortStateCounts, the same comparator breakdownByState uses). It backs the
// build-time SEO endpoint's status breakdown and total: RecentByAuthority renders
// the bounded list, this spans the whole partition, and the handler sums these
// buckets for the exact Total. A row whose appState is absent (Cosmos omits an
// undefined projection) OR explicit JSON null folds into the single nil-*string
// bucket, matching breakdownByState's nil semantics. It runs on the
// latency-tolerant build-read budget (QueryItemsLongRead), like the sibling SEO
// reads, and reads only from Cosmos (GH#395 Invariant 1) — never PlanIt.
func (s *CosmosStore) BreakdownByAuthority(ctx context.Context, authorityCode string) ([]StateCount, error) {
	// Latency-tolerant build-time read: a LARGE authority partition legitimately
	// exceeds the 1.5s OLTP budget, so use the longer per-attempt budget (tc-9tov).
	raws, err := s.items.QueryItemsLongRead(ctx, authorityCode, breakdownByAuthorityQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("status breakdown for authority %q: %w", authorityCode, err)
	}
	// breakdownRow decodes a GROUP BY projection. AppState is a *string so an
	// absent property (Cosmos omits undefined projections) and an explicit JSON
	// null both decode to nil — folded into the single nil bucket below.
	type breakdownRow struct {
		AppState *string `json:"appState"`
		Count    int     `json:"count"`
	}
	counts := make(map[string]int)
	nilCount := 0
	for _, raw := range raws {
		var row breakdownRow
		if err := json.Unmarshal(raw, &row); err != nil {
			return nil, fmt.Errorf("decode status breakdown row for authority %q: %w", authorityCode, err)
		}
		if row.AppState == nil {
			nilCount += row.Count
			continue
		}
		counts[*row.AppState] += row.Count
	}

	out := make([]StateCount, 0, len(counts)+1)
	for state, n := range counts {
		s := state
		out = append(out, StateCount{AppState: &s, Count: n})
	}
	if nilCount > 0 {
		out = append(out, StateCount{AppState: nil, Count: nilCount})
	}

	sortStateCounts(out)
	return out, nil
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

// countNearbyQuery is the exact, single-partition count of applications within
// radiusMetres of a point. It marries the recentNearbyQuery ST_DISTANCE filter
// (GeoJSON [longitude, latitude] order, all values bound as named parameters)
// with a SELECT VALUE COUNT(1) scalar instead of a bounded TOP @cap read, so it
// returns the EXACT in-radius total for the town SEO page. Cosmos returns a
// single row that is a bare JSON number.
const countNearbyQuery = "SELECT VALUE COUNT(1) FROM c WHERE ST_DISTANCE(c.location, " +
	`{"type": "Point", "coordinates": [@longitude, @latitude]}) <= @radiusMetres`

// CountNearby returns the exact number of applications within radiusMetres of
// (lat, lng) inside the authorityCode partition. It backs the build-time
// town-level SEO endpoint's total: RecentNearby renders the bounded list, this
// counts everything in radius. Coordinates and radius are bound as named
// parameters (not string-concatenated). It runs on the latency-tolerant
// build-read budget (QueryItemsLongRead) and reads only from Cosmos (GH#395
// Invariant 1) — never PlanIt.
func (s *CosmosStore) CountNearby(ctx context.Context, authorityCode string, lat, lng, radiusMetres float64) (int, error) {
	params := map[string]any{
		"@latitude":     lat,
		"@longitude":    lng,
		"@radiusMetres": radiusMetres,
	}
	// Latency-tolerant build-time read: a LARGE authority partition legitimately
	// exceeds the 1.5s OLTP budget, so use the longer per-attempt budget (tc-9tov).
	raws, err := s.items.QueryItemsLongRead(ctx, authorityCode, countNearbyQuery, params)
	if err != nil {
		return 0, fmt.Errorf("count applications near %q: %w", authorityCode, err)
	}
	if len(raws) == 0 {
		return 0, nil
	}
	var total int
	if err := json.Unmarshal(raws[0], &total); err != nil {
		return 0, fmt.Errorf("decode nearby application count for %q: %w", authorityCode, err)
	}
	return total, nil
}

// recentNearbyQuery is the bounded, single-partition spatial top-N query behind
// the build-time town-level SEO endpoint: the most recently active applications
// within radiusMetres of a point, inside one authorityCode partition. It marries
// the findNearbyQuery ST_DISTANCE filter (GeoJSON [longitude, latitude] order,
// all values bound as named parameters) with the recentByAuthorityQuery
// SELECT TOP @cap ... ORDER BY c.lastDifferent DESC discipline, so the read stays
// bounded by @cap and ordered by the index-backed lastDifferent field. Ordering
// is NOT by startDate: it is excluded from the indexing policy, so ordering by it
// would force a full-partition scan. This is a distinct query from FindNearby —
// the authed nearby path is deliberately left unchanged.
const recentNearbyQuery = "SELECT TOP @cap * FROM c WHERE ST_DISTANCE(c.location, " +
	`{"type": "Point", "coordinates": [@longitude, @latitude]}) <= @radiusMetres ` +
	"ORDER BY c.lastDifferent DESC"

// RecentNearby returns up to cap most-recently-active applications within
// radiusMetres of (lat, lng) inside the authorityCode partition, ordered by
// lastDifferent DESC. It backs the build-time town-level SEO endpoint: a single
// bounded single-partition spatial query, never a cross-partition fan-out and
// never an unbounded scan. Coordinates, radius, and cap are bound as named
// parameters (not string-concatenated). There is no PlanIt fallback (GH#395
// Invariant 1) — it reads only from Cosmos.
func (s *CosmosStore) RecentNearby(ctx context.Context, authorityCode string, lat, lng, radiusMetres float64, cap int) ([]PlanningApplication, error) {
	params := map[string]any{
		"@latitude":     lat,
		"@longitude":    lng,
		"@radiusMetres": radiusMetres,
		"@cap":          cap,
	}
	// Latency-tolerant build-time read: a LARGE authority partition legitimately
	// exceeds the 1.5s OLTP budget, so use the longer per-attempt budget (tc-9tov).
	raws, err := s.items.QueryItemsLongRead(ctx, authorityCode, recentNearbyQuery, params)
	if err != nil {
		return nil, fmt.Errorf("recent applications near %q: %w", authorityCode, err)
	}
	apps := make([]PlanningApplication, 0, len(raws))
	for _, raw := range raws {
		var doc applicationDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("decode recent nearby application in %q: %w", authorityCode, err)
		}
		apps = append(apps, doc.toDomain())
	}
	return apps, nil
}
