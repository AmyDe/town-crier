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

// storeSurface is the full method set the Applications store exposes to its
// consumers across packages (handler's appStore, recent.go's recentStore,
// near.go's nearStore, watchzones.appFinder's FindNearbyPage, the savedapplications
// snapshot backfill's GetByUID, and the poll Upsert). It exists only as a
// compile-time parity check: both *CosmosStore and *PostgresStore must satisfy it,
// so a Postgres port can never silently diverge from the Cosmos surface.
type storeSurface interface {
	Upsert(ctx context.Context, a PlanningApplication) error
	GetByAuthorityAndName(ctx context.Context, authorityCode, name string) (PlanningApplication, bool, error)
	GetByUID(ctx context.Context, uid, authorityCode string) (PlanningApplication, bool, error)
	RecentByAuthority(ctx context.Context, authorityCode string, cap int) ([]PlanningApplication, error)
	BreakdownByAuthority(ctx context.Context, authorityCode string) ([]StateCount, error)
	FindNearbyPage(ctx context.Context, latitude, longitude, radiusMetres float64, limit int, cursor string) ([]PlanningApplication, string, error)
	RecentNearby(ctx context.Context, authorityCode string, lat, lng, radiusMetres float64, cap int) ([]PlanningApplication, error)
	NearestNearby(ctx context.Context, authorityCode string, lat, lng, radiusMetres float64, cap int) ([]PlanningApplication, error)
	BreakdownNearby(ctx context.Context, authorityCode string, lat, lng, radiusMetres float64) ([]StateCount, error)
}

// Compile-time parity: the Postgres store satisfies the same consumer-side
// interfaces the Cosmos store does, and both satisfy the full storeSurface.
var (
	_ appStore     = (*PostgresStore)(nil)
	_ recentStore  = (*PostgresStore)(nil)
	_ nearStore    = (*PostgresStore)(nil)
	_ storeSurface = (*PostgresStore)(nil)
	_ storeSurface = (*CosmosStore)(nil)
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

const pgRecentByAuthorityQuery = "SELECT " + appColumns +
	" FROM applications WHERE authority_code = $1 ORDER BY last_different DESC LIMIT $2"

// RecentByAuthority returns up to cap most-recently-active applications in the
// authority, ordered by last_different DESC. It backs the build-time SEO endpoint.
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
	if err != nil {
		return nil, "", fmt.Errorf("find applications near (%v, %v): %w", latitude, longitude, err)
	}

	collected, err := pgx.CollectRows(rows, scanNearbyRow)
	if err != nil {
		return nil, "", fmt.Errorf("find applications near (%v, %v): %w", latitude, longitude, err)
	}

	apps := make([]PlanningApplication, len(collected))
	for i := range collected {
		apps[i] = collected[i].app
	}

	next := ""
	if limit > 0 && len(collected) == limit {
		last := collected[len(collected)-1]
		next, err = encodeNearbyCursor(last.dist, last.app.Name)
		if err != nil {
			return nil, "", fmt.Errorf("encode nearby cursor: %w", err)
		}
	}
	return apps, next, nil
}

// seoNearbyPoint is the query point for the authority-scoped SEO spatial reads,
// built from $2 (longitude) and $3 (latitude); $1 is the authority_code.
const seoNearbyPoint = "ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography"

const pgRecentNearbyQuery = "SELECT " + appColumns +
	" FROM applications WHERE authority_code = $1 AND ST_DWithin(location, " + seoNearbyPoint + ", $4) " +
	"ORDER BY last_different DESC LIMIT $5"

// RecentNearby returns up to cap most-recently-active applications within
// radiusMetres of (lat, lng) inside the authority, ordered by last_different DESC.
// The authority scope is kept for parity with the Cosmos read (memo 0010 notes a
// later phase may relax it; not relaxed here).
func (s *PostgresStore) RecentNearby(ctx context.Context, authorityCode string, lat, lng, radiusMetres float64, cap int) ([]PlanningApplication, error) {
	rows, err := s.db.Query(ctx, pgRecentNearbyQuery, authorityCode, lng, lat, radiusMetres, cap)
	if err != nil {
		return nil, fmt.Errorf("recent applications near %q: %w", authorityCode, err)
	}
	apps, err := pgx.CollectRows(rows, scanAppRow)
	if err != nil {
		return nil, fmt.Errorf("recent applications near %q: %w", authorityCode, err)
	}
	return apps, nil
}

const pgNearestNearbyQuery = "SELECT " + appColumns +
	" FROM applications WHERE authority_code = $1 AND ST_DWithin(location, " + seoNearbyPoint + ", $4) " +
	"ORDER BY location <-> " + seoNearbyPoint + " LIMIT $5"

// NearestNearby returns up to cap applications nearest to (lat, lng) within
// radiusMetres inside the authority, ordered by the KNN <-> distance ASC.
func (s *PostgresStore) NearestNearby(ctx context.Context, authorityCode string, lat, lng, radiusMetres float64, cap int) ([]PlanningApplication, error) {
	rows, err := s.db.Query(ctx, pgNearestNearbyQuery, authorityCode, lng, lat, radiusMetres, cap)
	if err != nil {
		return nil, fmt.Errorf("nearest applications near %q: %w", authorityCode, err)
	}
	apps, err := pgx.CollectRows(rows, scanAppRow)
	if err != nil {
		return nil, fmt.Errorf("nearest applications near %q: %w", authorityCode, err)
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
