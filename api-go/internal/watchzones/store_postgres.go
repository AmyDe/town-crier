package watchzones

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// pgUniqueViolationCode is the Postgres SQLSTATE for a unique-constraint
// violation. Save uses it to detect a name collision against the
// watch_zones_user_id_name_key constraint (GH#1083, tc-h4y98), mirroring
// internal/offercodes/store_postgres.go's identical helper.
const pgUniqueViolationCode = "23505"

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgUniqueViolationCode
	}
	return false
}

// querier is the consumer-side slice of *pgxpool.Pool the store uses. Defining it
// here keeps the store decoupled from the concrete pool; both *pgxpool.Pool and
// pgx.Tx satisfy it structurally.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Store is the full watch-zone store method set its consumers rely on. It is the
// exported consumer-side interface cmd/api's newRouter accepts for the watch-zone
// routes. The narrower per-handler interfaces (zoneStore, zoneAuthorityLister,
// demoaccount.zoneStore) are all subsets of this set.
type Store interface {
	GetByUserID(ctx context.Context, userID string) ([]WatchZone, error)
	Get(ctx context.Context, userID, zoneID string) (WatchZone, error)
	Save(ctx context.Context, z WatchZone) error
	Delete(ctx context.Context, userID, zoneID string) error
	DeleteAllByUserID(ctx context.Context, userID string) error
	FindZonesContaining(ctx context.Context, latitude, longitude float64) ([]WatchZone, error)
	All(ctx context.Context) ([]WatchZone, error)
}

// Compile-time check: the store satisfies the consumer-side Store interface.
var _ Store = (*PostgresStore)(nil)

// PostgresStore reads and writes watch zones in the Postgres `watch_zones` table
// (Cosmos -> Postgres + PostGIS migration; memo 0010, epic #645). It is a parallel
// implementation: Cosmos remains wired, so nothing here is on a live path yet.
//
// The notify hot path, FindZonesContaining, becomes a single ST_DWithin against
// one GiST index across every user's zones — authority-agnostic by construction,
// with no bounding-box prune or cross-partition fan-out.
type PostgresStore struct {
	db querier
}

// NewPostgresStore returns a store over the given pgx pool (or any querier).
func NewPostgresStore(db querier) *PostgresStore {
	return &PostgresStore{db: db}
}

// pgZoneColumns is the read projection. id is rendered as text; ST_Y is the
// latitude and ST_X the longitude of the (NOT NULL) geography point;
// ST_AsGeoJSON(boundary) is NULL for a circle zone and a GeoJSON Polygon for a
// custom-shape one. The order MUST match scanZone.
const pgZoneColumns = "id::text, user_id, name, ST_Y(location::geometry), " +
	"ST_X(location::geometry), radius_metres, created_at, " +
	"push_enabled, email_instant_enabled, ST_AsGeoJSON(boundary)"

// scanZone hydrates one zone through NewWatchZone, so the same invariants the
// domain enforces (positive radius, non-blank id/user/name) gate a row read
// from the database. A non-NULL boundary column is decoded from GeoJSON and
// re-validated through NewBoundary (via decodeBoundaryGeoJSON), applying the
// same domain invariants to a boundary read back from the database as to one
// supplied by a client.
func scanZone(row pgx.Row) (WatchZone, error) {
	var (
		id, userID, name       string
		latitude, longitude    float64
		radiusMetres           float64
		createdAt              time.Time
		pushEnabled, emailFlag bool
		boundaryGeoJSON        *string
	)
	if err := row.Scan(&id, &userID, &name, &latitude, &longitude, &radiusMetres,
		&createdAt, &pushEnabled, &emailFlag, &boundaryGeoJSON); err != nil {
		return WatchZone{}, err
	}
	zone, err := NewWatchZone(id, userID, name, latitude, longitude, radiusMetres,
		createdAt, pushEnabled, emailFlag)
	if err != nil {
		return WatchZone{}, err
	}
	if boundaryGeoJSON != nil {
		boundary, err := decodeBoundaryGeoJSON(*boundaryGeoJSON)
		if err != nil {
			return WatchZone{}, fmt.Errorf("hydrate watch zone %q boundary: %w", id, err)
		}
		zone.Boundary = boundary
	}
	return zone, nil
}

// boundaryGeoJSON is the GeoJSON Polygon wire representation of a Boundary
// ring: a single exterior ring, [longitude, latitude] coordinate order
// (GeoJSON/RFC 7946 convention). It is shared by two layers: this store, which
// round-trips it as a JSON string for ST_GeomFromGeoJSON (write) and
// ST_AsGeoJSON (read), and the HTTP handlers (handler.go, nearby.go) plus the
// GDPR export adapter (cmd/api/export_adapters.go), which round-trip it
// directly as a JSON value in request/response bodies. Single outer ring only
// -- no holes, no multi-polygon, mirroring the domain's Boundary type.
type boundaryGeoJSON struct {
	Type        string         `json:"type"`
	Coordinates [][][2]float64 `json:"coordinates"`
}

// boundaryToGeoJSON renders b as its GeoJSON Polygon wire representation, or
// nil for a circle zone (nil/empty Boundary) -- the "no shape" value at every
// layer that uses this type, matching a NULL boundary column here and a null
// "boundary" JSON field at the HTTP layer.
func boundaryToGeoJSON(b Boundary) *boundaryGeoJSON {
	if len(b) == 0 {
		return nil
	}
	ring := make([][2]float64, len(b))
	for i, v := range b {
		ring[i] = [2]float64{v.Longitude, v.Latitude}
	}
	return &boundaryGeoJSON{Type: "Polygon", Coordinates: [][][2]float64{ring}}
}

// vertices extracts the outer ring as raw, UNVALIDATED Coordinates -- callers
// must pass them through NewBoundary (directly, or via WithBoundary /
// WithUpdates) before trusting them as a valid shape.
func (g boundaryGeoJSON) vertices() []Coordinate {
	if len(g.Coordinates) == 0 {
		return nil
	}
	ring := g.Coordinates[0]
	out := make([]Coordinate, len(ring))
	for i, pt := range ring {
		out[i] = Coordinate{Longitude: pt[0], Latitude: pt[1]}
	}
	return out
}

// encodeBoundaryGeoJSON renders b as the GeoJSON text ST_GeomFromGeoJSON(...)
// expects. A nil or empty Boundary (a circle zone) encodes to a nil *string,
// which the caller binds as SQL NULL so ST_GeomFromGeoJSON(NULL)::geography
// also yields NULL -- clearing any previously-persisted boundary on update.
func encodeBoundaryGeoJSON(b Boundary) (*string, error) {
	g := boundaryToGeoJSON(b)
	if g == nil {
		return nil, nil
	}
	raw, err := json.Marshal(g)
	if err != nil {
		return nil, fmt.Errorf("encode boundary geojson: %w", err)
	}
	s := string(raw)
	return &s, nil
}

// decodeBoundaryGeoJSON parses ST_AsGeoJSON(boundary)'s text back into a
// Boundary, re-validating it through NewBoundary. raw is never the empty
// string in practice -- scanZone only calls this when the boundary column
// scanned non-NULL.
func decodeBoundaryGeoJSON(raw string) (Boundary, error) {
	var g boundaryGeoJSON
	if err := json.Unmarshal([]byte(raw), &g); err != nil {
		return nil, fmt.Errorf("decode boundary geojson: %w", err)
	}
	vertices := g.vertices()
	if len(vertices) == 0 {
		return nil, nil
	}
	return NewBoundary(vertices)
}

// scanZoneRow adapts scanZone to pgx.CollectRows over a multi-row result.
func scanZoneRow(row pgx.CollectableRow) (WatchZone, error) {
	return scanZone(row)
}

const pgGetByUserIDQuery = "SELECT " + pgZoneColumns +
	" FROM watch_zones WHERE user_id = $1 ORDER BY id"

// GetByUserID returns all of the user's zones, ordered by id for determinism.
func (s *PostgresStore) GetByUserID(ctx context.Context, userID string) ([]WatchZone, error) {
	rows, err := s.db.Query(ctx, pgGetByUserIDQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("query watch zones for %q: %w", userID, err)
	}
	zones, err := pgx.CollectRows(rows, scanZoneRow)
	if err != nil {
		return nil, fmt.Errorf("query watch zones for %q: %w", userID, err)
	}
	return zones, nil
}

const pgGetZoneQuery = "SELECT " + pgZoneColumns +
	" FROM watch_zones WHERE user_id = $1 AND id = $2::uuid"

// Get point-reads a single zone. A miss surfaces as ErrNotFound.
func (s *PostgresStore) Get(ctx context.Context, userID, zoneID string) (WatchZone, error) {
	z, err := scanZone(s.db.QueryRow(ctx, pgGetZoneQuery, userID, zoneID))
	if errors.Is(err, pgx.ErrNoRows) {
		return WatchZone{}, ErrNotFound
	}
	if err != nil {
		return WatchZone{}, fmt.Errorf("read watch zone %q: %w", zoneID, err)
	}
	return z, nil
}

const pgSaveZoneQuery = `
INSERT INTO watch_zones (
	id, user_id, name, location, radius_metres,
	push_enabled, email_instant_enabled, created_at, boundary
) VALUES (
	$1::uuid, $2, $3, ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography,
	$6, $7, $8, $9, ST_GeomFromGeoJSON($10)::geography
)
ON CONFLICT (id) DO UPDATE SET
	user_id = EXCLUDED.user_id,
	name = EXCLUDED.name,
	location = EXCLUDED.location,
	radius_metres = EXCLUDED.radius_metres,
	push_enabled = EXCLUDED.push_enabled,
	email_instant_enabled = EXCLUDED.email_instant_enabled,
	created_at = EXCLUDED.created_at,
	boundary = EXCLUDED.boundary`

// Save upserts the zone keyed on its uuid id, creating it or overwriting every
// column -- including boundary -- on an existing row. A nil/empty z.Boundary
// (a circle zone) writes SQL NULL, so saving a zone that previously had a
// custom shape with Boundary cleared correctly reverts the persisted row to a
// circle rather than leaving a stale polygon behind.
//
// ON CONFLICT (id) only dedupes an upsert of the SAME id -- it does not catch
// a name collision against a different, already-existing row for the same
// user (watch_zones also has UNIQUE (user_id, name)). create always mints a
// fresh id, so that case reaches Postgres as a raw unique-violation on
// watch_zones_user_id_name_key; isUniqueViolation translates it into the
// domain sentinel ErrDuplicateName instead (GH#1083, tc-h4y98).
func (s *PostgresStore) Save(ctx context.Context, z WatchZone) error {
	boundaryGeoJSON, err := encodeBoundaryGeoJSON(z.Boundary)
	if err != nil {
		return fmt.Errorf("encode watch zone %q boundary: %w", z.ID, err)
	}
	_, err = s.db.Exec(ctx, pgSaveZoneQuery,
		z.ID, z.UserID, z.Name, z.Longitude, z.Latitude, z.RadiusMetres,
		z.PushEnabled, z.EmailInstantEnabled, z.CreatedAt, boundaryGeoJSON,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateName
		}
		return fmt.Errorf("upsert watch zone %q: %w", z.ID, err)
	}
	return nil
}

const pgDeleteZoneQuery = "DELETE FROM watch_zones WHERE user_id = $1 AND id = $2::uuid"

// Delete removes a zone. A miss surfaces as ErrNotFound.
func (s *PostgresStore) Delete(ctx context.Context, userID, zoneID string) error {
	tag, err := s.db.Exec(ctx, pgDeleteZoneQuery, userID, zoneID)
	if err != nil {
		return fmt.Errorf("delete watch zone %q: %w", zoneID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

const pgDeleteAllByUserIDQuery = "DELETE FROM watch_zones WHERE user_id = $1"

// DeleteAllByUserID removes every watch zone owned by the user (account-deletion
// cascade). Deleting zero rows is not an error.
func (s *PostgresStore) DeleteAllByUserID(ctx context.Context, userID string) error {
	if _, err := s.db.Exec(ctx, pgDeleteAllByUserIDQuery, userID); err != nil {
		return fmt.Errorf("delete all watch zones for %q: %w", userID, err)
	}
	return nil
}

const pgAllZonesQuery = "SELECT " + pgZoneColumns + " FROM watch_zones ORDER BY id"

// All returns every watch zone across every user, ordered by id for
// determinism. It backs the dev-seed job's zone-geometry read (bd tc-9nbs4.1,
// GH#1076 Phase 1): the same "every zone dev currently has" input the now-removed
// DistinctAuthorityIDs previously served as a set of authority ids, now served
// as full zone geometries, so devseed can query prod by geography instead of
// authority id -- notification matching is purely geographic (ADR 0041/0044),
// so a watch zone no longer needs a "home" authority to scope the read.
func (s *PostgresStore) All(ctx context.Context) ([]WatchZone, error) {
	rows, err := s.db.Query(ctx, pgAllZonesQuery)
	if err != nil {
		return nil, fmt.Errorf("query all watch zones: %w", err)
	}
	zones, err := pgx.CollectRows(rows, scanZoneRow)
	if err != nil {
		return nil, fmt.Errorf("query all watch zones: %w", err)
	}
	return zones, nil
}

const pgFindZonesContainingQuery = "SELECT " + pgZoneColumns +
	" FROM watch_zones WHERE " +
	"(boundary IS NULL AND ST_DWithin(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, radius_metres)) " +
	"OR (boundary IS NOT NULL AND ST_Covers(boundary, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography)) " +
	"ORDER BY id"

// FindZonesContaining returns every watch zone (across all users, all authorities)
// that contains the point (latitude, longitude): a circle zone (boundary IS NULL)
// via ST_DWithin against its centre and radius, a custom-shape zone (boundary IS
// NOT NULL) via ST_Covers against its polygon. ST_Covers, not ST_Intersects or
// ST_Contains, is deliberate: the application is a point, and ST_Covers includes a
// point exactly on the boundary edge while being well-defined for geography. Each
// branch is served by its own GiST index (watch_zones_location_gist,
// watch_zones_boundary_gist) — the notify hot path. Zones are hydrated through
// NewWatchZone.
func (s *PostgresStore) FindZonesContaining(ctx context.Context, latitude, longitude float64) ([]WatchZone, error) {
	rows, err := s.db.Query(ctx, pgFindZonesContainingQuery, longitude, latitude)
	if err != nil {
		return nil, fmt.Errorf("find zones containing point: %w", err)
	}
	zones, err := pgx.CollectRows(rows, scanZoneRow)
	if err != nil {
		return nil, fmt.Errorf("find zones containing point: %w", err)
	}
	return zones, nil
}
