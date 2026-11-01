package applications

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// querier is the consumer-side slice of *pgxpool.Pool the store uses: parameterised
// exec/query/query-row plus BeginTx. Defining it here (not importing pgxpool) keeps
// the store decoupled from the concrete pool. *pgxpool.Pool and *pgxpool.Conn
// satisfy it structurally; a pgx.Tx does not.
type querier interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Store is the full method set the Applications store exposes to its consumers
// across packages (handler's appStore, watchzones.appFinder's FindNearbyPage, the savedapplications snapshot
// backfill's GetByUID, and the poll
// Upsert). It is the exported consumer-side interface cmd/api's newRouter accepts
// for the Applications routes. Every narrower per-handler interface is a subset
// of this set.
type Store interface {
	Upsert(ctx context.Context, a PlanningApplication) error
	GetByAuthorityAndName(ctx context.Context, authorityCode, name string) (PlanningApplication, bool, error)
	GetByUID(ctx context.Context, uid, authorityCode string) (PlanningApplication, bool, error)
	RecentByAuthority(ctx context.Context, authorityCode string, cap int) ([]PlanningApplication, error)
	FindNearbyPage(ctx context.Context, latitude, longitude, radiusMetres float64, limit int, cursor string) ([]PlanningApplication, string, error)
	RecentNearPoint(ctx context.Context, latitude, longitude, radiusMetres float64, limit int) ([]PlanningApplication, error)
	FindInZonePage(ctx context.Context, q InZoneQuery) ([]PlanningApplication, string, error)
	FindClustersInZone(ctx context.Context, q ClusterQuery) ([]Cluster, error)
	FindInBoundaryPage(ctx context.Context, latitude, longitude float64, boundary []Coordinate, limit int, cursor string) ([]PlanningApplication, string, error)
	FindClustersInBoundary(ctx context.Context, q BoundaryClusterQuery) ([]Cluster, error)
	FindInBoundaryZonePage(ctx context.Context, q InBoundaryQuery) ([]PlanningApplication, string, error)
	Search(ctx context.Context, query, authorityCode string, lat, lon float64, limit int) ([]PlanningApplication, bool, error)
}

var (
	_ appStore    = (*PostgresStore)(nil)
	_ searchStore = (*PostgresStore)(nil)
	_ Store       = (*PostgresStore)(nil)
)

// PostgresStore reads and writes planning applications in the Postgres
// `applications` table.
//
//   - The natural key is the COMPOSITE (authority_code, planit_name) — a PlanIt
//     case reference is only unique within an authority — so Upsert is
//     INSERT ... ON CONFLICT (authority_code, planit_name) DO UPDATE.
//   - location is a geography(Point,4326) served by one GiST index, so the radius
//     reads use ST_DWithin and the nearest-first read uses the KNN <-> operator.
type PostgresStore struct {
	db     querier
	logger *slog.Logger
}

// NewPostgresStore returns a store over the given pgx pool (or any querier).
func NewPostgresStore(db querier) *PostgresStore {
	return &PostgresStore{db: db, logger: slog.New(slog.DiscardHandler)}
}

// WithLogger sets the logger used for store warnings and returns the store.
func (s *PostgresStore) WithLogger(logger *slog.Logger) *PostgresStore {
	s.logger = logger
	return s
}

// withNoNestLoop runs fn in a read-only transaction with nested loops disabled.
// The planner estimates the cluster CTE at one row and picks a nested loop that
// re-reads it per cell; queries inside fn must use pgx.QueryExecModeExec, because
// a cached generic plan ignores SET LOCAL. Rows must be fully collected inside fn.
func (s *PostgresStore) withNoNestLoop(ctx context.Context, fn func(pgx.Tx) error) error {
	return s.withLocalSetting(ctx, "SET LOCAL enable_nestloop = off", fn)
}

// withKNNWalk runs fn in a read-only transaction with bitmap scans disabled. On a
// dense zone the planner estimates fewer rows than the LIMIT and reads the whole
// zone through a bitmap scan instead of walking the GiST index nearest-first;
// queries inside fn must use pgx.QueryExecModeExec, because a cached generic plan
// ignores SET LOCAL. Rows must be fully collected inside fn.
func (s *PostgresStore) withKNNWalk(ctx context.Context, fn func(pgx.Tx) error) error {
	return s.withLocalSetting(ctx, "SET LOCAL enable_bitmapscan = off", fn)
}

func (s *PostgresStore) withLocalSetting(ctx context.Context, setStmt string, fn func(pgx.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("begin read-only transaction: %w", err)
	}
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := tx.Rollback(rollbackCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			s.logger.WarnContext(rollbackCtx, "rollback read-only transaction", slog.Any("error", err))
		}
	}()

	if _, err := tx.Exec(ctx, setStmt); err != nil {
		return fmt.Errorf("%s: %w", setStmt, err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit read-only transaction: %w", err)
	}
	return nil
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
// a NULL location.
// $20-$26 are the PlanIt full-field widening's seven silent columns (GH#935):
// other_fields ($26) binds a map[string]any natively as jsonb via pgx; a nil
// map binds as SQL NULL. altid/associated_id ($21/$22) bind raw JSON bytes the
// same way; a nil slice binds as SQL NULL.
const upsertQuery = `
INSERT INTO applications (
	planit_name, authority_code, uid, area_name, area_id, address, postcode,
	description, app_type, app_state, app_size, start_date, decided_date,
	consulted_date, location, url, link, last_different,
	reference, altid, associated_id, last_changed, last_scraped, scraper_name, other_fields
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
	ST_SetSRID(ST_MakePoint($15::double precision, $16::double precision), 4326)::geography,
	$17, $18, $19,
	$20, $21, $22, $23, $24, $25, $26
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
	last_different = EXCLUDED.last_different,
	reference = EXCLUDED.reference,
	altid = EXCLUDED.altid,
	associated_id = EXCLUDED.associated_id,
	last_changed = EXCLUDED.last_changed,
	last_scraped = EXCLUDED.last_scraped,
	scraper_name = EXCLUDED.scraper_name,
	other_fields = EXCLUDED.other_fields`

// Upsert inserts or updates the application. authority_code is the stringified
// AreaID.
func (s *PostgresStore) Upsert(ctx context.Context, a PlanningApplication) error {
	_, err := s.db.Exec(ctx, upsertQuery,
		a.Name, strconv.Itoa(a.AreaID), a.UID, a.AreaName, a.AreaID, a.Address,
		a.Postcode, a.Description, a.AppType, a.AppState, a.AppSize,
		a.StartDate, a.DecidedDate, a.ConsultedDate,
		a.Longitude, a.Latitude, a.URL, a.Link, a.LastDifferent,
		a.Reference, nilIfEmptyBytes(a.Altid), nilIfEmptyBytes(a.AssociatedID),
		a.LastChanged, a.LastScraped, a.ScraperName, nilIfEmptyOtherFields(a.OtherFields),
	)
	if err != nil {
		return fmt.Errorf("upsert application %q: %w", a.Name, err)
	}
	return nil
}

// nilIfEmptyBytes returns b, or an explicit nil when b is empty, so pgx binds
// SQL NULL rather than an empty jsonb value for an absent Altid/AssociatedID.
func nilIfEmptyBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	return b
}

// nilIfEmptyOtherFields returns fields, or an explicit nil when fields is
// empty, so pgx binds SQL NULL rather than jsonb "{}" for an absent
// other_fields map.
func nilIfEmptyOtherFields(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	return fields
}

const getByAuthorityAndNameQuery = "SELECT " + appColumns +
	" FROM applications WHERE authority_code = $1 AND planit_name = $2"

// The planit_name LIKE predicate must stay in the query text so the planner can
// use the partial applications_collapsed_slash_name index.
const getByCollapsedNameQuery = "SELECT " + appColumns +
	" FROM applications WHERE authority_code = $1 AND planit_name LIKE '%//%'" +
	" AND regexp_replace(planit_name, '/{2,}', '/', 'g') = $2 LIMIT 2"

// GetByAuthorityAndName point-reads by (authority_code, planit_name). The boolean
// reports presence; a miss is a normal 404 for the caller, not an error.
//
// The edge merges runs of slashes in request paths, so a stored name containing
// "//" arrives collapsed. On an exact miss, when name contains "/" but no "//",
// the lookup falls back to the stored name with slash runs collapsed. Exactly
// one match is returned; an ambiguous match (two or more) is reported as not
// found and logged.
func (s *PostgresStore) GetByAuthorityAndName(ctx context.Context, authorityCode, name string) (PlanningApplication, bool, error) {
	a, err := scanApp(s.db.QueryRow(ctx, getByAuthorityAndNameQuery, authorityCode, name))
	if errors.Is(err, pgx.ErrNoRows) {
		if strings.Contains(name, "/") && !strings.Contains(name, "//") {
			return s.getByCollapsedName(ctx, authorityCode, name)
		}
		return PlanningApplication{}, false, nil
	}
	if err != nil {
		return PlanningApplication{}, false, fmt.Errorf("read application %q/%q: %w", authorityCode, name, err)
	}
	return a, true, nil
}

func (s *PostgresStore) getByCollapsedName(ctx context.Context, authorityCode, name string) (PlanningApplication, bool, error) {
	rows, err := s.db.Query(ctx, getByCollapsedNameQuery, authorityCode, name)
	if err != nil {
		return PlanningApplication{}, false, fmt.Errorf("read application %q/%q by collapsed name: %w", authorityCode, name, err)
	}
	defer rows.Close()

	var matches []PlanningApplication
	for rows.Next() {
		a, err := scanApp(rows)
		if err != nil {
			return PlanningApplication{}, false, fmt.Errorf("scan application %q/%q by collapsed name: %w", authorityCode, name, err)
		}
		matches = append(matches, a)
	}
	if err := rows.Err(); err != nil {
		return PlanningApplication{}, false, fmt.Errorf("read application %q/%q by collapsed name: %w", authorityCode, name, err)
	}
	switch len(matches) {
	case 0:
		return PlanningApplication{}, false, nil
	case 1:
		return matches[0], true, nil
	default:
		s.logger.WarnContext(ctx, "ambiguous collapsed application name",
			"authorityCode", authorityCode, "name", name)
		return PlanningApplication{}, false, nil
	}
}

// fullAppColumns is appColumns plus the seven silent-field columns (GH#935,
// the PlanIt full-field widening). It is used ONLY by getByUIDQuery: the
// ingester's read-back needs the silent-field snapshot to evaluate
// HasSameSilentFieldsAs. Every other read keeps appColumns unchanged, so
// list/search payloads and every HTTP response DTO are byte-for-byte
// unaffected — no query path outside GetByUID ever selects these columns.
const fullAppColumns = appColumns +
	", reference, altid, associated_id, last_changed, last_scraped, scraper_name, other_fields"

const getByUIDQuery = "SELECT " + fullAppColumns +
	" FROM applications WHERE authority_code = $1 AND uid = $2 ORDER BY planit_name LIMIT 1"

// scanFullApp hydrates one application from a fullAppColumns single-row read:
// appScanDest's destinations plus the seven silent fields. other_fields scans
// into an intermediate []byte (the jsonb wire form) and is then unmarshalled,
// matching the savedapplications snapshot-column pattern.
func scanFullApp(row pgx.Row) (PlanningApplication, error) {
	var a PlanningApplication
	var otherFieldsJSON []byte
	dest := append(appScanDest(&a),
		&a.Reference, &a.Altid, &a.AssociatedID, &a.LastChanged, &a.LastScraped, &a.ScraperName, &otherFieldsJSON)
	if err := row.Scan(dest...); err != nil {
		return PlanningApplication{}, err
	}
	if len(otherFieldsJSON) > 0 {
		if err := json.Unmarshal(otherFieldsJSON, &a.OtherFields); err != nil {
			return PlanningApplication{}, fmt.Errorf("decode other_fields: %w", err)
		}
	}
	return a, nil
}

// GetByUID looks up an application by its raw PlanIt uid within an authority,
// for the saved-application lazy snapshot backfill and the ingester's
// silent-field read-back. The boolean reports presence.
func (s *PostgresStore) GetByUID(ctx context.Context, uid, authorityCode string) (PlanningApplication, bool, error) {
	a, err := scanFullApp(s.db.QueryRow(ctx, getByUIDQuery, authorityCode, uid))
	if errors.Is(err, pgx.ErrNoRows) {
		return PlanningApplication{}, false, nil
	}
	if err != nil {
		return PlanningApplication{}, false, fmt.Errorf("read application uid %q in %q: %w", uid, authorityCode, err)
	}
	return a, true, nil
}

const matchesExpressionQuery = "SELECT to_tsvector('english', $1::text) @@ to_tsquery('english', $2)"

// MatchesExpression reports whether description matches the given
// to_tsquery('english', ...) boolean expression under Postgres full-text
// search — the watch-zone pre-canned filter matcher (GH#1090, epic tc-w825j;
// see watchzones.FilterDefinition.Expression for the catalog this is built
// to evaluate). It touches no table: both operands are literals, so there is
// no index or row cost to this call beyond the tsvector/tsquery parse.
func (s *PostgresStore) MatchesExpression(ctx context.Context, description, expression string) (bool, error) {
	var matches bool
	if err := s.db.QueryRow(ctx, matchesExpressionQuery, description, expression).Scan(&matches); err != nil {
		return false, fmt.Errorf("match description against expression %q: %w", expression, err)
	}
	return matches, nil
}

// RecentRealDateOrder is the shared "sort by the stable real-world lifecycle
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
// value. Shared by pgRecentByAuthorityQuery and pgRecentNearPointQuery.
const RecentRealDateOrder = "GREATEST(decided_date, start_date) DESC NULLS LAST, start_date DESC NULLS LAST, planit_name"

const pgRecentByAuthorityQuery = "SELECT " + appColumns +
	" FROM applications WHERE authority_code = $1 ORDER BY " + RecentRealDateOrder + " LIMIT $2"

// RecentByAuthority returns up to cap most-recently-active applications in the
// authority, ordered by RecentRealDateOrder. It backs the live SEO authority page.
func (s *PostgresStore) RecentByAuthority(ctx context.Context, authorityCode string, cap int) ([]PlanningApplication, error) {
	apps, err := s.queryApps(ctx, pgRecentByAuthorityQuery, authorityCode, cap)
	if err != nil {
		return nil, fmt.Errorf("recent applications for authority %q: %w", authorityCode, err)
	}
	return apps, nil
}

// nearbyPoint is the GeoJSON-equivalent query point for the authority-agnostic
// browse reads, built from $1 (longitude) and $2 (latitude).
const nearbyPoint = "ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography"

// knnThenExact returns the nearest-first query over filter. The planit_name
// tie-break blocks the GiST KNN walk, so the knn CTE orders by distance alone to
// find dmax, the distance of the limit-th nearest match. At least limit matches
// lie within dmax, so the outer ST_DWithin keeps every row of the exact
// (distance, planit_name) top limit. <-> measures on the sphere but ST_DWithin on
// the spheroid (up to ~0.34% longer), hence the 1% + 1 m margin.
func knnThenExact(filter, limitParam string) string {
	return "WITH knn AS MATERIALIZED (SELECT location <-> " + nearbyPoint + " AS d FROM applications WHERE " + filter +
		" ORDER BY location <-> " + nearbyPoint + " LIMIT " + limitParam + ") " +
		"SELECT " + appColumns + ", location <-> " + nearbyPoint + " AS distance FROM applications WHERE " + filter +
		" AND ST_DWithin(location, " + nearbyPoint + ", (SELECT max(d) FROM knn) * 1.01 + 1)" +
		" ORDER BY location <-> " + nearbyPoint + ", planit_name LIMIT " + limitParam
}

var findNearbyFirstPageQuery = knnThenExact("ST_DWithin(location, "+nearbyPoint+", $3)", "$4")

// findNearbyKeysetQuery resumes strictly after the cursor's (distance, planit_name).
// The keyset predicate sits in both steps so dmax is measured over the remaining rows.
var findNearbyKeysetQuery = knnThenExact("ST_DWithin(location, "+nearbyPoint+", $3) "+
	"AND (location <-> "+nearbyPoint+" > $5 "+
	"OR (location <-> "+nearbyPoint+" = $5 AND planit_name > $6))", "$4")

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
// (empty when exhausted), using KNN ordering with keyset pagination. It is
// authority-agnostic.
func (s *PostgresStore) FindNearbyPage(ctx context.Context, latitude, longitude, radiusMetres float64, limit int, cursor string) ([]PlanningApplication, string, error) {
	query, args := findNearbyFirstPageQuery, []any{longitude, latitude, radiusMetres, limit}
	if cursor != "" {
		dist, name, decodeErr := decodeNearbyCursor(cursor)
		if decodeErr != nil {
			return nil, "", fmt.Errorf("decode nearby cursor: %w", decodeErr)
		}
		query, args = findNearbyKeysetQuery, append(args, dist, name)
	}
	wrap := func(err error) error {
		return fmt.Errorf("find applications near (%v, %v): %w", latitude, longitude, err)
	}
	var (
		apps []PlanningApplication
		next string
	)
	err := s.withKNNWalk(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, query, append([]any{pgx.QueryExecModeExec}, args...)...)
		if err != nil {
			return wrap(err)
		}
		apps, next, err = collectPage(rows, scanNearbyRow, nearbyAppOf,
			func(last nearbyRow) (string, error) {
				enc, err := encodeNearbyCursor(last.dist, last.app.Name)
				if err != nil {
					return "", fmt.Errorf("encode nearby cursor: %w", err)
				}
				return enc, nil
			},
			limit, wrap)
		return err
	})
	if err != nil {
		return nil, "", err
	}
	return apps, next, nil
}

// pgRecentNearPointQuery mirrors pgRecentByAuthorityQuery's RecentRealDateOrder
// but is authority-agnostic and radius-bounded (ST_DWithin over nearbyPoint,
// exactly like findNearbyFirstPageQuery) instead of authority-scoped — the
// public near-point browse endpoint's ?sort=recent (GH#912 Phase 2). It returns
// a single ordered page with NO keyset continuation, unlike FindNearbyPage:
// RecentRealDateOrder's primary key, GREATEST(decided_date, start_date), and its
// secondary key, start_date, are each independently nullable, so a correct
// keyset predicate needs a per-NULL-combination branch (see statusKeysetQuery
// and its three siblings in zonepage.go for the shape that takes) for an
// ordering nothing has needed to paginate before now — RecentByAuthority, the other
// RecentRealDateOrder call site, is a single-page cap-only read too. The near-point recent sort's only consumer
// (the anonymous browse list, GH#912 Phase 3) fetches one bounded page
// (<= nearPointMaxLimit), so this mirrors that existing single-page shape
// rather than building an unused continuation.
const pgRecentNearPointQuery = "SELECT " + appColumns +
	" FROM applications WHERE ST_DWithin(location, " + nearbyPoint + ", $3) " +
	"ORDER BY " + RecentRealDateOrder + " LIMIT $4"

// pgRecentNearPointSinceQuery adds a lower bound on the sort key so the planner
// can AND the applications_real_date bitmap with the spatial one and fetch only
// recent heap rows: an unbounded read in dense London does one random page read
// per in-radius row, which exceeds nearPointTimeout on a cold cache. $5 is cast
// to date so the predicate matches the index expression.
const pgRecentNearPointSinceQuery = "SELECT " + appColumns +
	" FROM applications WHERE ST_DWithin(location, " + nearbyPoint + ", $3) " +
	"AND GREATEST(decided_date, start_date) >= $5::date " +
	"ORDER BY " + RecentRealDateOrder + " LIMIT $4"

// recentNearPointWindowDays are tried in order before the unbounded query. A
// window that fills the limit returns exactly the unbounded top-N, because
// every excluded row sorts after every included one.
var recentNearPointWindowDays = []int{30, 90, 365}

// RecentNearPoint returns up to limit applications within radiusMetres of
// (latitude, longitude), ordered by RecentRealDateOrder (most-recently-decided,
// falling back to most-recently-submitted, NULLS LAST) — the near-point browse
// endpoint's ?sort=recent (GH#912 Phase 2). It is authority-agnostic like
// FindNearbyPage, which remains the sole paginated (?sort=distance) path; see
// pgRecentNearPointQuery for why this one does not paginate.
func (s *PostgresStore) RecentNearPoint(ctx context.Context, latitude, longitude, radiusMetres float64, limit int) ([]PlanningApplication, error) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	for _, days := range recentNearPointWindowDays {
		since := today.AddDate(0, 0, -days).Format(time.DateOnly)
		apps, err := s.queryApps(ctx, pgRecentNearPointSinceQuery, pgx.QueryExecModeExec, longitude, latitude, radiusMetres, limit, since)
		if err != nil {
			return nil, fmt.Errorf("recent applications near (%v, %v) since %s: %w", latitude, longitude, since, err)
		}
		if len(apps) >= limit {
			return apps, nil
		}
	}
	apps, err := s.queryApps(ctx, pgRecentNearPointQuery, pgx.QueryExecModeExec, longitude, latitude, radiusMetres, limit)
	if err != nil {
		return nil, fmt.Errorf("recent applications near (%v, %v): %w", latitude, longitude, err)
	}
	return apps, nil
}

func (s *PostgresStore) queryApps(ctx context.Context, query string, args ...any) ([]PlanningApplication, error) {
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanAppRow)
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
