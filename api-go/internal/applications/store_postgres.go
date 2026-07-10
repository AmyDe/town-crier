package applications

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// querier is the consumer-side slice of *pgxpool.Pool the store uses: parameterised
// exec/query/query-row. Defining it here (not importing pgxpool) keeps the store
// decoupled from the concrete pool and lets a pgx.Tx stand in. Both *pgxpool.Pool
// and pgx.Tx satisfy it structurally.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Store is the full method set the Applications store exposes to its consumers
// across packages (handler's appStore, recent.go's recentStore, near.go's
// nearStore, watchzones.appFinder's FindNearbyPage, the savedapplications snapshot
// backfill's GetByUID, demoaccount's Upsert + FindNearbyPage, and the poll
// Upsert). It is the exported consumer-side interface cmd/api's newRouter accepts
// for the Applications routes. Every narrower per-handler interface is a subset
// of this set.
type Store interface {
	Upsert(ctx context.Context, a PlanningApplication) error
	GetByAuthorityAndName(ctx context.Context, authorityCode, name string) (PlanningApplication, bool, error)
	GetByUID(ctx context.Context, uid, authorityCode string) (PlanningApplication, bool, error)
	RecentByAuthority(ctx context.Context, authorityCode string, cap int) ([]PlanningApplication, error)
	BreakdownByAuthority(ctx context.Context, authorityCode string) ([]StateCount, error)
	FindNearbyPage(ctx context.Context, latitude, longitude, radiusMetres float64, limit int, cursor string) ([]PlanningApplication, string, error)
	RecentNearPoint(ctx context.Context, latitude, longitude, radiusMetres float64, limit int) ([]PlanningApplication, error)
	FindInZonePage(ctx context.Context, q InZoneQuery) ([]PlanningApplication, string, error)
	FindClustersInZone(ctx context.Context, q ClusterQuery) ([]Cluster, error)
	RecentNearestTown(ctx context.Context, authorityCode string, lat, lng, radiusMetres float64, siblings []TownCentroid, cap int) ([]PlanningApplication, error)
	BreakdownNearby(ctx context.Context, authorityCode string, lat, lng, radiusMetres float64) ([]StateCount, error)
	Search(ctx context.Context, query, authorityCode string, limit int) ([]PlanningApplication, bool, error)
}

// Compile-time check: the Postgres store satisfies the per-handler consumer-side
// interfaces and the full Store surface.
var (
	_ appStore    = (*PostgresStore)(nil)
	_ recentStore = (*PostgresStore)(nil)
	_ nearStore   = (*PostgresStore)(nil)
	_ searchStore = (*PostgresStore)(nil)
	_ Store       = (*PostgresStore)(nil)
)

// PostgresStore reads and writes planning applications in the Postgres
// `applications` table (Cosmos -> Postgres + PostGIS migration; memo 0010, epic
// #645). It is a parallel implementation: Cosmos remains the wired datastore, so
// nothing here is on a live path yet.
//
// Key design vs the Cosmos store:
//   - The natural key is the COMPOSITE (authority_code, planit_name) — a PlanIt
//     case reference is only unique within an authority — so Upsert is
//     INSERT ... ON CONFLICT (authority_code, planit_name) DO UPDATE.
//   - location is a geography(Point,4326) served by one GiST index, so the radius
//     reads use ST_DWithin and the nearest-first read uses the KNN <-> operator —
//     the true nearest-N ordering the Cosmos Gateway refuses cross-partition.
type PostgresStore struct {
	db querier
}

// NewPostgresStore returns a store over the given pgx pool (or any querier).
func NewPostgresStore(db querier) *PostgresStore {
	return &PostgresStore{db: db}
}

// appColumns is the read projection. Its order MUST match appScanDest. ST_Y is the
// latitude and ST_X the longitude of the geography point (NULL when location is
// absent); the geometry cast is required for the accessor functions.
const appColumns = "planit_name, uid, area_name, area_id, address, postcode, " +
	"description, app_type, app_state, app_size, start_date, decided_date, " +
	"consulted_date, ST_Y(location::geometry), ST_X(location::geometry), url, link, last_different"

// appScanDest returns the scan destinations for appColumns, in order. The
// nullable text/float/date columns scan into pointer fields (NULL -> nil).
func appScanDest(a *PlanningApplication) []any {
	return []any{
		&a.Name, &a.UID, &a.AreaName, &a.AreaID, &a.Address, &a.Postcode,
		&a.Description, &a.AppType, &a.AppState, &a.AppSize,
		&a.StartDate, &a.DecidedDate, &a.ConsultedDate,
		&a.Latitude, &a.Longitude, &a.URL, &a.Link, &a.LastDifferent,
	}
}

// scanApp hydrates one application from a single-row read (QueryRow).
func scanApp(row pgx.Row) (PlanningApplication, error) {
	var a PlanningApplication
	if err := row.Scan(appScanDest(&a)...); err != nil {
		return PlanningApplication{}, err
	}
	return a, nil
}

// scanAppRow adapts scanApp to pgx.CollectRows over a multi-row result.
func scanAppRow(row pgx.CollectableRow) (PlanningApplication, error) {
	return scanApp(row)
}

// upsertQuery writes the application keyed on the composite (authority_code,
// planit_name). location is built from longitude ($15) and latitude ($16): when
// either is NULL, ST_MakePoint yields NULL, so a coordinate-less application stores
// a NULL location — matching the Cosmos newGeoPoint both-or-nothing rule.
const upsertQuery = `
INSERT INTO applications (
	planit_name, authority_code, uid, area_name, area_id, address, postcode,
	description, app_type, app_state, app_size, start_date, decided_date,
	consulted_date, location, url, link, last_different
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
	ST_SetSRID(ST_MakePoint($15::double precision, $16::double precision), 4326)::geography,
	$17, $18, $19
)
ON CONFLICT (authority_code, planit_name) DO UPDATE SET
	uid = EXCLUDED.uid,
	area_name = EXCLUDED.area_name,
	area_id = EXCLUDED.area_id,
	address = EXCLUDED.address,
	postcode = EXCLUDED.postcode,
	description = EXCLUDED.description,
	app_type = EXCLUDED.app_type,
	app_state = EXCLUDED.app_state,
	app_size = EXCLUDED.app_size,
	start_date = EXCLUDED.start_date,
	decided_date = EXCLUDED.decided_date,
	consulted_date = EXCLUDED.consulted_date,
	location = EXCLUDED.location,
	url = EXCLUDED.url,
	link = EXCLUDED.link,
	last_different = EXCLUDED.last_different`

// Upsert inserts or updates the application. authority_code is the stringified
// AreaID, matching the Cosmos partition key.
func (s *PostgresStore) Upsert(ctx context.Context, a PlanningApplication) error {
	_, err := s.db.Exec(ctx, upsertQuery,
		a.Name, strconv.Itoa(a.AreaID), a.UID, a.AreaName, a.AreaID, a.Address,
		a.Postcode, a.Description, a.AppType, a.AppState, a.AppSize,
		a.StartDate, a.DecidedDate, a.ConsultedDate,
		a.Longitude, a.Latitude, a.URL, a.Link, a.LastDifferent,
	)
	if err != nil {
		return fmt.Errorf("upsert application %q: %w", a.Name, err)
	}
	return nil
}

const getByAuthorityAndNameQuery = "SELECT " + appColumns +
	" FROM applications WHERE authority_code = $1 AND planit_name = $2"

// GetByAuthorityAndName point-reads by (authority_code, planit_name). The boolean
// reports presence; a miss is a normal 404 for the caller, not an error.
func (s *PostgresStore) GetByAuthorityAndName(ctx context.Context, authorityCode, name string) (PlanningApplication, bool, error) {
	a, err := scanApp(s.db.QueryRow(ctx, getByAuthorityAndNameQuery, authorityCode, name))
	if errors.Is(err, pgx.ErrNoRows) {
		return PlanningApplication{}, false, nil
	}
	if err != nil {
		return PlanningApplication{}, false, fmt.Errorf("read application %q/%q: %w", authorityCode, name, err)
	}
	return a, true, nil
}

const getByUIDQuery = "SELECT " + appColumns +
	" FROM applications WHERE authority_code = $1 AND uid = $2 ORDER BY planit_name LIMIT 1"

// GetByUID looks up an application by its raw PlanIt uid within an authority,
// for the saved-application lazy snapshot backfill. The boolean reports presence.
func (s *PostgresStore) GetByUID(ctx context.Context, uid, authorityCode string) (PlanningApplication, bool, error) {
	a, err := scanApp(s.db.QueryRow(ctx, getByUIDQuery, authorityCode, uid))
	if errors.Is(err, pgx.ErrNoRows) {
		return PlanningApplication{}, false, nil
	}
	if err != nil {
		return PlanningApplication{}, false, fmt.Errorf("read application uid %q in %q: %w", uid, authorityCode, err)
	}
	return a, true, nil
}

// recentRealDateOrder is the shared "sort by the stable real-world lifecycle
// date, not PlanIt's last_different re-index marker" ORDER BY clause (#819
// decision 1). last_different is bumped to "now" whenever PlanIt re-indexes an
// application regardless of its actual age, which floats stale applications to
// the top of a last_different-ordered list — GREATEST(decided_date, start_date)
// never moves on a re-index. Postgres's GREATEST ignores NULL arguments (unlike
// last_different, which is NOT NULL), so a decided application sorts by its
// decided_date, an undecided one by its start_date, and an application with
// neither sorts last (NULLS LAST) rather than erroring or floating to the top.
// The tie-break (start_date DESC NULLS LAST, then planit_name ASC) makes the
// order fully deterministic when two applications share the same GREATEST
// value. Shared by pgRecentByAuthorityQuery and pgRecentNearestTownQuery.
const recentRealDateOrder = "GREATEST(decided_date, start_date) DESC NULLS LAST, start_date DESC NULLS LAST, planit_name"

const pgRecentByAuthorityQuery = "SELECT " + appColumns +
	" FROM applications WHERE authority_code = $1 ORDER BY " + recentRealDateOrder + " LIMIT $2"

// RecentByAuthority returns up to cap most-recently-active applications in the
// authority, ordered by recentRealDateOrder. It backs the build-time SEO endpoint.
func (s *PostgresStore) RecentByAuthority(ctx context.Context, authorityCode string, cap int) ([]PlanningApplication, error) {
	rows, err := s.db.Query(ctx, pgRecentByAuthorityQuery, authorityCode, cap)
	if err != nil {
		return nil, fmt.Errorf("recent applications for authority %q: %w", authorityCode, err)
	}
	apps, err := pgx.CollectRows(rows, scanAppRow)
	if err != nil {
		return nil, fmt.Errorf("recent applications for authority %q: %w", authorityCode, err)
	}
	return apps, nil
}

const pgRecentInAuthoritiesQuery = "SELECT " + appColumns +
	" FROM applications WHERE area_id = ANY($1) ORDER BY last_different DESC LIMIT $2"

// RecentInAuthorities returns up to limit most-recently-active applications
// across all of authorityIDs (matched against the numeric area_id, not the
// stringified authority_code), ordered by last_different DESC. It backs the
// dev-seed job's prod read (epic #808): a small, cross-authority "most recently
// changed" window, not a per-authority scoped read like RecentByAuthority.
//
// A nil or empty authorityIDs is a normal no-op (e.g. dev has no watch zones
// yet) and returns an empty slice with no error — pgx correctly binds an empty
// []int to ANY($1) as an empty array, matching zero rows, rather than erroring
// or panicking.
func (s *PostgresStore) RecentInAuthorities(ctx context.Context, authorityIDs []int, limit int) ([]PlanningApplication, error) {
	rows, err := s.db.Query(ctx, pgRecentInAuthoritiesQuery, authorityIDs, limit)
	if err != nil {
		return nil, fmt.Errorf("recent applications in authorities %v: %w", authorityIDs, err)
	}
	apps, err := pgx.CollectRows(rows, scanAppRow)
	if err != nil {
		return nil, fmt.Errorf("recent applications in authorities %v: %w", authorityIDs, err)
	}
	return apps, nil
}

const pgBreakdownByAuthorityQuery = "SELECT app_state, count(*) FROM applications " +
	"WHERE authority_code = $1 GROUP BY app_state"

// BreakdownByAuthority returns the per-app_state distribution over the whole
// authority, ordered by sortStateCounts (count DESC, then app_state ASC, nil last).
// A NULL app_state is its own GROUP BY bucket and folds into the single nil bucket.
func (s *PostgresStore) BreakdownByAuthority(ctx context.Context, authorityCode string) ([]StateCount, error) {
	rows, err := s.db.Query(ctx, pgBreakdownByAuthorityQuery, authorityCode)
	if err != nil {
		return nil, fmt.Errorf("status breakdown for authority %q: %w", authorityCode, err)
	}
	out, err := collectBreakdown(rows)
	if err != nil {
		return nil, fmt.Errorf("status breakdown for authority %q: %w", authorityCode, err)
	}
	return out, nil
}

// nearbyPoint is the GeoJSON-equivalent query point for the authority-agnostic
// browse reads, built from $1 (longitude) and $2 (latitude).
const nearbyPoint = "ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography"

// findNearbyFirstPageQuery returns the nearest `limit` in-radius applications,
// ordered by the KNN <-> distance then planit_name (the keyset tie-break).
const findNearbyFirstPageQuery = "SELECT " + appColumns + ", location <-> " + nearbyPoint + " AS distance " +
	"FROM applications WHERE ST_DWithin(location, " + nearbyPoint + ", $3) " +
	"ORDER BY location <-> " + nearbyPoint + ", planit_name LIMIT $4"

// findNearbyKeysetQuery resumes after the cursor's (distance, planit_name): rows
// strictly farther, or at the same distance with a greater name. This is the
// stable keyset predicate matching the ORDER BY, so pages never overlap or gap.
const findNearbyKeysetQuery = "SELECT " + appColumns + ", location <-> " + nearbyPoint + " AS distance " +
	"FROM applications WHERE ST_DWithin(location, " + nearbyPoint + ", $3) " +
	"AND (location <-> " + nearbyPoint + " > $5 " +
	"OR (location <-> " + nearbyPoint + " = $5 AND planit_name > $6)) " +
	"ORDER BY location <-> " + nearbyPoint + ", planit_name LIMIT $4"

// nearbyRow carries a hydrated application plus its computed distance, so the last
// row of a full page can be encoded into the next-page cursor.
type nearbyRow struct {
	app  PlanningApplication
	dist float64
}

func scanNearbyRow(row pgx.CollectableRow) (nearbyRow, error) {
	var nr nearbyRow
	dest := append(appScanDest(&nr.app), &nr.dist)
	if err := row.Scan(dest...); err != nil {
		return nearbyRow{}, err
	}
	return nr, nil
}

// FindNearbyPage returns one nearest-first page of up to limit applications within
// radiusMetres of (latitude, longitude), plus an opaque cursor for the next page
// (empty when exhausted). This is the new, correct nearest-first behaviour memo
// 0010 calls for — true KNN ordering with stable keyset pagination, which the
// Cosmos Gateway cannot serve cross-partition. It is authority-agnostic.
func (s *PostgresStore) FindNearbyPage(ctx context.Context, latitude, longitude, radiusMetres float64, limit int, cursor string) ([]PlanningApplication, string, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if cursor == "" {
		rows, err = s.db.Query(ctx, findNearbyFirstPageQuery, longitude, latitude, radiusMetres, limit)
	} else {
		dist, name, decodeErr := decodeNearbyCursor(cursor)
		if decodeErr != nil {
			return nil, "", fmt.Errorf("decode nearby cursor: %w", decodeErr)
		}
		rows, err = s.db.Query(ctx, findNearbyKeysetQuery, longitude, latitude, radiusMetres, limit, dist, name)
	}
	wrap := func(err error) error {
		return fmt.Errorf("find applications near (%v, %v): %w", latitude, longitude, err)
	}
	if err != nil {
		return nil, "", wrap(err)
	}
	return collectPage(rows, scanNearbyRow, nearbyAppOf,
		func(last nearbyRow) (string, error) {
			enc, err := encodeNearbyCursor(last.dist, last.app.Name)
			if err != nil {
				return "", fmt.Errorf("encode nearby cursor: %w", err)
			}
			return enc, nil
		},
		limit, wrap)
}

// pgRecentNearPointQuery mirrors pgRecentByAuthorityQuery's recentRealDateOrder
// but is authority-agnostic and radius-bounded (ST_DWithin over nearbyPoint,
// exactly like findNearbyFirstPageQuery) instead of authority-scoped — the
// public near-point browse endpoint's ?sort=recent (GH#912 Phase 2). It returns
// a single ordered page with NO keyset continuation, unlike FindNearbyPage:
// recentRealDateOrder's primary key, GREATEST(decided_date, start_date), and its
// secondary key, start_date, are each independently nullable, so a correct
// keyset predicate needs a per-NULL-combination branch (see statusKeysetQuery
// and its three siblings in zonepage.go for the shape that takes) for an
// ordering nothing has needed to paginate before now — RecentByAuthority and
// RecentNearestTown, the two other recentRealDateOrder call sites, are both
// single-page cap-only reads too. The near-point recent sort's only consumer
// (the anonymous browse list, GH#912 Phase 3) fetches one bounded page
// (<= nearPointMaxLimit), so this mirrors that existing single-page shape
// rather than building an unused continuation.
const pgRecentNearPointQuery = "SELECT " + appColumns +
	" FROM applications WHERE ST_DWithin(location, " + nearbyPoint + ", $3) " +
	"ORDER BY " + recentRealDateOrder + " LIMIT $4"

// RecentNearPoint returns up to limit applications within radiusMetres of
// (latitude, longitude), ordered by recentRealDateOrder (most-recently-decided,
// falling back to most-recently-submitted, NULLS LAST) — the near-point browse
// endpoint's ?sort=recent (GH#912 Phase 2). It is authority-agnostic like
// FindNearbyPage, which remains the sole paginated (?sort=distance) path; see
// pgRecentNearPointQuery for why this one does not paginate.
func (s *PostgresStore) RecentNearPoint(ctx context.Context, latitude, longitude, radiusMetres float64, limit int) ([]PlanningApplication, error) {
	rows, err := s.db.Query(ctx, pgRecentNearPointQuery, longitude, latitude, radiusMetres, limit)
	if err != nil {
		return nil, fmt.Errorf("recent applications near (%v, %v): %w", latitude, longitude, err)
	}
	apps, err := pgx.CollectRows(rows, scanAppRow)
	if err != nil {
		return nil, fmt.Errorf("recent applications near (%v, %v): %w", latitude, longitude, err)
	}
	return apps, nil
}

// seoNearbyPoint is the query point for the authority-scoped SEO spatial reads,
// built from $2 (longitude) and $3 (latitude); $1 is the authority_code.
const seoNearbyPoint = "ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography"

// pgRecentNearestTownQuery is the town-level Voronoi partition read (#819
// decisions 2-3). The `towns` CTE puts the target town's own point/radius at
// idx 0 ($2 lng, $3 lat, $4 radius) and zips the sibling arrays ($5 lngs, $6
// lats, $7 radii) via one WITH-ORDINALITY unnest into idx 1..N — so the query
// text never changes shape however many siblings are passed; zero siblings
// means unnest yields zero rows and the read degrades to a plain, non-
// partitioned radius read for the target town alone.
//
// `candidates` joins every authority-scoped application against every town it
// is within THAT TOWN'S OWN radius of (ST_DWithin(a.location, t.pt, t.radius))
// — this is the in-range-nearest rule already taking effect: a town whose
// radius can't reach an application never produces a candidate row for it, so
// that application is invisible to it regardless of how much nearer its centroid
// is. `nearest` then keeps exactly one row per planit_name — the covering town
// with the smallest KNN distance (DISTINCT ON, ties broken toward the lowest
// idx for determinism) — which is precisely "closest-wins among the towns that
// can reach it" and guarantees single-assignment: no application can ever
// satisfy `idx = 0` for two different requests (this town's and a sibling's).
//
// The final SELECT keeps only rows assigned to the target town (idx = 0) and
// re-applies recentRealDateOrder (decision 1) so a town's own list is ordered
// exactly like the authority list.
const pgRecentNearestTownQuery = `
WITH towns AS (
	SELECT 0 AS idx,
	       ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography AS pt,
	       $4::double precision AS radius
	UNION ALL
	SELECT ord::int,
	       ST_SetSRID(ST_MakePoint(lng, lat), 4326)::geography,
	       radius
	FROM unnest($5::double precision[], $6::double precision[], $7::double precision[])
	     WITH ORDINALITY AS sib(lng, lat, radius, ord)
),
candidates AS (
	SELECT
		a.planit_name, a.uid, a.area_name, a.area_id, a.address, a.postcode,
		a.description, a.app_type, a.app_state, a.app_size, a.start_date,
		a.decided_date, a.consulted_date, a.location, a.url, a.link, a.last_different,
		t.idx, a.location <-> t.pt AS dist
	FROM applications a
	JOIN towns t ON ST_DWithin(a.location, t.pt, t.radius)
	WHERE a.authority_code = $1
),
nearest AS (
	SELECT DISTINCT ON (planit_name) *
	FROM candidates
	ORDER BY planit_name, dist ASC, idx ASC
)
SELECT
	planit_name, uid, area_name, area_id, address, postcode, description,
	app_type, app_state, app_size, start_date, decided_date, consulted_date,
	ST_Y(location::geometry), ST_X(location::geometry), url, link, last_different
FROM nearest
WHERE idx = 0
ORDER BY ` + recentRealDateOrder + `
LIMIT $8`

// RecentNearestTown returns up to cap applications assigned to THIS town by the
// query-time Voronoi partition (#819 decisions 2-3): authority-scoped, kept
// only if this town's own (lat, lng, radiusMetres) or one of siblings' own
// (lat, lng, radiusMetres) covers it, and assigned to whichever covering town
// is nearest — so an application whose true-nearest centroid can't reach it,
// but a farther sibling's wider radius can, lands on that farther sibling
// instead of being orphaned (the in-range-nearest rule). siblings is every
// OTHER gazetteer town centroid (each with its own radius) in the same
// authority; a nil/empty siblings degrades to a plain radius read for this
// town alone. Ordered by GREATEST(decided_date, start_date) DESC NULLS LAST
// (decision 1), tie-broken by start_date DESC NULLS LAST, planit_name.
// Authority pages are NOT partitioned (decision 4) — RecentByAuthority is
// unaffected by this method.
func (s *PostgresStore) RecentNearestTown(ctx context.Context, authorityCode string, lat, lng, radiusMetres float64, siblings []TownCentroid, cap int) ([]PlanningApplication, error) {
	lngs := make([]float64, len(siblings))
	lats := make([]float64, len(siblings))
	radii := make([]float64, len(siblings))
	for i, c := range siblings {
		lngs[i] = c.Lng
		lats[i] = c.Lat
		radii[i] = c.RadiusMetres
	}
	rows, err := s.db.Query(ctx, pgRecentNearestTownQuery,
		authorityCode, lng, lat, radiusMetres, lngs, lats, radii, cap)
	if err != nil {
		return nil, fmt.Errorf("recent applications nearest town in %q: %w", authorityCode, err)
	}
	apps, err := pgx.CollectRows(rows, scanAppRow)
	if err != nil {
		return nil, fmt.Errorf("recent applications nearest town in %q: %w", authorityCode, err)
	}
	return apps, nil
}

const pgBreakdownNearbyQuery = "SELECT app_state, count(*) FROM applications " +
	"WHERE authority_code = $1 AND ST_DWithin(location, " + seoNearbyPoint + ", $4) GROUP BY app_state"

// BreakdownNearby returns the per-app_state distribution over the in-radius,
// authority-scoped set, ordered by sortStateCounts. The NULL bucket folds into nil.
func (s *PostgresStore) BreakdownNearby(ctx context.Context, authorityCode string, lat, lng, radiusMetres float64) ([]StateCount, error) {
	rows, err := s.db.Query(ctx, pgBreakdownNearbyQuery, authorityCode, lng, lat, radiusMetres)
	if err != nil {
		return nil, fmt.Errorf("status breakdown near %q: %w", authorityCode, err)
	}
	out, err := collectBreakdown(rows)
	if err != nil {
		return nil, fmt.Errorf("status breakdown near %q: %w", authorityCode, err)
	}
	return out, nil
}

// collectBreakdown scans (app_state, count) rows into StateCounts (NULL app_state
// -> nil bucket) and applies the shared sortStateCounts ordering.
func collectBreakdown(rows pgx.Rows) ([]StateCount, error) {
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (StateCount, error) {
		var sc StateCount
		if err := row.Scan(&sc.AppState, &sc.Count); err != nil {
			return StateCount{}, err
		}
		return sc, nil
	})
	if err != nil {
		return nil, err
	}
	sortStateCounts(out)
	return out, nil
}

// nearbyCursor is the opaque keyset cursor: the last row's distance (as a
// round-trippable decimal string) and planit_name. It is base64url-encoded JSON.
type nearbyCursor struct {
	D string `json:"d"`
	N string `json:"n"`
}

// encodeNearbyCursor serialises the keyset position. The distance is formatted
// with shortest round-trippable precision so the next page's predicate compares
// against the exact float the database produced.
func encodeNearbyCursor(dist float64, name string) (string, error) {
	b, err := json.Marshal(nearbyCursor{D: strconv.FormatFloat(dist, 'g', -1, 64), N: name})
	if err != nil {
		return "", fmt.Errorf("marshal cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func decodeNearbyCursor(cursor string) (dist float64, name string, err error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, "", fmt.Errorf("base64 decode: %w", err)
	}
	var c nearbyCursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return 0, "", fmt.Errorf("unmarshal cursor: %w", err)
	}
	dist, err = strconv.ParseFloat(c.D, 64)
	if err != nil {
		return 0, "", fmt.Errorf("parse cursor distance: %w", err)
	}
	return dist, c.N, nil
}
