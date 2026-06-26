package applications

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// CosmosItems is the consumer-side slice of the Applications container the store
// uses: a single-partition point read, an upsert, single-partition queries, and
// a bounded cross-partition spatial fan-out. QueryItems carries the tight 1.5s
// OLTP per-attempt budget for user-facing reads; QueryItemsLongRead carries a
// longer per-attempt budget for the latency-tolerant build-time SEO reads over a
// LARGE authority partition (tc-9tov); QueryPageCrossPartition backs the bounded,
// cursor-paged authority-agnostic nearby fan-out (tc-fm8f, replacing the
// unbounded drain from tc-zldl). platform.CosmosContainer satisfies it
// structurally.
type CosmosItems interface {
	ReadItem(ctx context.Context, partitionKey, id string) ([]byte, error)
	UpsertItem(ctx context.Context, partitionKey string, item []byte) error
	QueryItems(ctx context.Context, partitionKey, query string, params map[string]any) ([][]byte, error)
	QueryItemsLongRead(ctx context.Context, partitionKey, query string, params map[string]any) ([][]byte, error)
	QueryItemsCrossPartition(ctx context.Context, query string, params map[string]any) ([][]byte, error)
	QueryPageCrossPartition(ctx context.Context, query string, params map[string]any, pageSize int, continuationToken string) ([][]byte, string, error)
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
// (sortStateCounts, the same comparator BreakdownNearby uses). It backs the
// build-time SEO endpoint's status breakdown and total: RecentByAuthority renders
// the bounded list, this spans the whole partition, and the handler sums these
// buckets for the exact Total. A row whose appState is absent (Cosmos omits an
// undefined projection) OR explicit JSON null folds into the single nil-*string
// bucket, matching BreakdownNearby's nil semantics. It runs on the
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

// findNearbyQuery is the constant-radius ST_DISTANCE spatial query for nearby
// applications. The radius is the query's constant, so the Applications
// /location spatial index serves it directly — cross-partition included.
// Coordinates and radius are bound as named parameters (mirroring
// findZonesContainingQuery in the watchzones package) — no float literals are
// concatenated into the query text.
const findNearbyQuery = "SELECT * FROM c WHERE ST_DISTANCE(c.location, " +
	`{"type": "Point", "coordinates": [@longitude, @latitude]}) <= @radiusMetres`

// FindNearby returns every application within radiusMetres of (latitude,
// longitude) via a constant-radius ST_DISTANCE spatial query against the GeoJSON
// location. Coordinates and radius are bound as named parameters (not
// string-concatenated) to eliminate float-formatting edge cases and to mirror
// the parameterized style of the sibling watchzones.FindZonesContaining query.
//
// This is a deliberate CROSS-PARTITION spatial fan-out: it is no longer scoped
// to one authorityCode partition, so a watch zone whose circle crosses an
// authority boundary surfaces in-circle applications on BOTH sides (tc-zldl /
// tc-w11n). It is user-initiated and low-frequency (zone create / zone open),
// strictly colder than the already-cross-partition notify path, so the fan-out
// RU is acceptable at current scale. The constant radius lets the /location
// spatial index serve the ST_DISTANCE residual exactly, so circle semantics
// stay exact even across partitions.
func (s *CosmosStore) FindNearby(ctx context.Context, latitude, longitude, radiusMetres float64) ([]PlanningApplication, error) {
	params := map[string]any{
		"@latitude":     latitude,
		"@longitude":    longitude,
		"@radiusMetres": radiusMetres,
	}
	raws, err := s.items.QueryItemsCrossPartition(ctx, findNearbyQuery, params)
	if err != nil {
		return nil, fmt.Errorf("find applications near (%v, %v): %w", latitude, longitude, err)
	}
	apps := make([]PlanningApplication, 0, len(raws))
	for _, raw := range raws {
		var doc applicationDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("decode nearby application near (%v, %v): %w", latitude, longitude, err)
		}
		apps = append(apps, doc.toDomain())
	}
	return apps, nil
}

// FindNearbyPage returns ONE bounded page of up to limit applications within
// radiusMetres of (latitude, longitude), plus an opaque continuation token for
// the next page (empty when the query is exhausted). It runs the same
// constant-radius ST_DISTANCE spatial query as FindNearby, served by the
// /location spatial index, and fans out cross-partition so a circle that crosses
// an authority boundary surfaces in-circle applications on BOTH sides (tc-zldl /
// tc-w11n). There is NO ORDER BY: the Cosmos Gateway rejects cross-partition
// ordering/aggregates, so a paged cross-partition result is in arbitrary
// (partition) order.
//
// The cap is applied AT THE QUERY LAYER via the page-size hint, so this issues
// exactly one gateway round-trip and never drains all pages — the fix for the
// unbounded fan-out that blew the server write timeout on dense urban zones
// (tc-fm8f). cursor resumes a prior page; "" starts at the first page.
// Coordinates and radius are bound as named parameters (not string-concatenated),
// mirroring the parameterized style of watchzones.FindZonesContaining.
func (s *CosmosStore) FindNearbyPage(ctx context.Context, latitude, longitude, radiusMetres float64, limit int, cursor string) ([]PlanningApplication, string, error) {
	params := map[string]any{
		"@latitude":     latitude,
		"@longitude":    longitude,
		"@radiusMetres": radiusMetres,
	}
	raws, next, err := s.items.QueryPageCrossPartition(ctx, findNearbyQuery, params, limit, cursor)
	if err != nil {
		return nil, "", fmt.Errorf("find applications near (%v, %v): %w", latitude, longitude, err)
	}
	apps := make([]PlanningApplication, 0, len(raws))
	for _, raw := range raws {
		var doc applicationDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, "", fmt.Errorf("decode nearby application near (%v, %v): %w", latitude, longitude, err)
		}
		apps = append(apps, doc.toDomain())
	}
	return apps, next, nil
}

// breakdownNearbyQuery is the exact, index-served per-appState distribution over
// the applications within radiusMetres of a point, inside a single authorityCode
// partition. It marries the recentNearbyQuery ST_DISTANCE filter (GeoJSON
// [longitude, latitude] order, all values bound as named parameters) with the
// breakdownByAuthorityQuery GROUP BY c.appState discipline instead of a scalar
// COUNT, so the buckets sum to the EXACT in-radius total — unlike the TOP-@cap
// bounded read, which saturates at the cap. The GROUP BY is served by the
// appState index over the in-radius set, so it hydrates no documents and stays
// cheap (on prod ~1.06x–1.18x the scalar ST_DISTANCE count it replaces, scaling
// linearly with the in-radius set). Each row is a projection {"appState": "...", "count": N};
// Cosmos OMITS the appState property entirely when the value is undefined, so a
// missing-appState bucket arrives without that key (decoded as a nil *string),
// distinct from an explicit JSON null.
const breakdownNearbyQuery = "SELECT c.appState, COUNT(1) AS count FROM c WHERE ST_DISTANCE(c.location, " +
	`{"type": "Point", "coordinates": [@longitude, @latitude]}) <= @radiusMetres ` +
	"GROUP BY c.appState"

// BreakdownNearby returns the per-appState distribution over the WHOLE set of
// applications within radiusMetres of (lat, lng) inside the authorityCode
// partition, ordered count DESC then appState ASC with nil last (sortStateCounts,
// the same comparator BreakdownByAuthority uses). It backs the build-time
// town-level SEO endpoint's status breakdown and total: RecentNearby (or
// NearestNearby) renders the bounded list, this spans the whole in-radius set,
// and the handler sums these buckets for the exact Total. A row whose appState is
// absent (Cosmos omits an undefined projection) OR explicit JSON null folds into
// the single nil-*string bucket, matching BreakdownByAuthority's nil semantics.
// Coordinates and radius are bound as named parameters (not string-concatenated).
// It runs on the latency-tolerant build-read budget (QueryItemsLongRead), like the
// sibling SEO reads, and reads only from Cosmos (GH#395 Invariant 1) — never PlanIt.
func (s *CosmosStore) BreakdownNearby(ctx context.Context, authorityCode string, lat, lng, radiusMetres float64) ([]StateCount, error) {
	params := map[string]any{
		"@latitude":     lat,
		"@longitude":    lng,
		"@radiusMetres": radiusMetres,
	}
	// Latency-tolerant build-time read: a LARGE authority partition legitimately
	// exceeds the 1.5s OLTP budget, so use the longer per-attempt budget (tc-9tov).
	raws, err := s.items.QueryItemsLongRead(ctx, authorityCode, breakdownNearbyQuery, params)
	if err != nil {
		return nil, fmt.Errorf("status breakdown near %q: %w", authorityCode, err)
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
			return nil, fmt.Errorf("decode status breakdown row near %q: %w", authorityCode, err)
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

// nearestNearbyQuery is the bounded, single-partition spatial top-N query behind
// the build-time town-level SEO endpoint's distance-ordered variant: the @cap
// applications NEAREST to a point, inside one authorityCode partition. It marries
// the recentNearbyQuery ST_DISTANCE filter (GeoJSON [longitude, latitude] order,
// all values bound as named parameters) with a SELECT TOP @cap ... ORDER BY
// ST_DISTANCE ASC ordering — so the read stays bounded by @cap and returns the
// nearest-first set, minimising overlap between adjacent town pages in
// conurbations. The ORDER BY repeats the same parameterized ST_DISTANCE
// expression as the WHERE filter; no coordinate is concatenated into the text.
// This is a sibling of recentNearbyQuery: that path (ordered by lastDifferent
// DESC) is the default and is deliberately left unchanged.
const nearestNearbyQuery = "SELECT TOP @cap * FROM c WHERE ST_DISTANCE(c.location, " +
	`{"type": "Point", "coordinates": [@longitude, @latitude]}) <= @radiusMetres ` +
	"ORDER BY ST_DISTANCE(c.location, " +
	`{"type": "Point", "coordinates": [@longitude, @latitude]})`

// NearestNearby returns up to cap applications NEAREST to (lat, lng) within
// radiusMetres inside the authorityCode partition, ordered by ST_DISTANCE ASC
// (nearest first). It backs the distance-ordered variant of the build-time
// town-level SEO endpoint: a single bounded single-partition spatial query, never
// a cross-partition fan-out and never an unbounded scan. Coordinates, radius, and
// cap are bound as named parameters (not string-concatenated). It runs on the
// latency-tolerant build-read budget (QueryItemsLongRead), like its RecentNearby
// sibling. There is no PlanIt fallback (GH#395 Invariant 1) — it reads only from
// Cosmos.
func (s *CosmosStore) NearestNearby(ctx context.Context, authorityCode string, lat, lng, radiusMetres float64, cap int) ([]PlanningApplication, error) {
	params := map[string]any{
		"@latitude":     lat,
		"@longitude":    lng,
		"@radiusMetres": radiusMetres,
		"@cap":          cap,
	}
	// Latency-tolerant build-time read: a LARGE authority partition legitimately
	// exceeds the 1.5s OLTP budget, so use the longer per-attempt budget (tc-9tov).
	raws, err := s.items.QueryItemsLongRead(ctx, authorityCode, nearestNearbyQuery, params)
	if err != nil {
		return nil, fmt.Errorf("nearest applications near %q: %w", authorityCode, err)
	}
	apps := make([]PlanningApplication, 0, len(raws))
	for _, raw := range raws {
		var doc applicationDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("decode nearest nearby application in %q: %w", authorityCode, err)
		}
		apps = append(apps, doc.toDomain())
	}
	return apps, nil
}
