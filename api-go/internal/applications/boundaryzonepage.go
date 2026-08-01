package applications

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// InBoundaryQuery is FindInBoundaryZonePage's request descriptor (GH#1031,
// tc-acbsh): the custom-shape zone's polygon (Boundary, a closed ring,
// [longitude, latitude]-ordered) replaces InZoneQuery's membership circle
// (RadiusMetres) as the containment test; Latitude/Longitude are the polygon's
// derived centroid, used ONLY to order SortDistance nearest-centroid-first
// (exactly as FindInBoundaryPage's do) -- containment is decided entirely by
// ST_Covers against Boundary. Sort/Status/Unread/Limit/Cursor carry the exact
// same meaning as InZoneQuery's, giving a custom-shape zone full sort/filter
// parity with a circle zone.
type InBoundaryQuery struct {
	UserID    string
	Latitude  float64
	Longitude float64
	Boundary  []Coordinate
	Sort      Sort
	Status    string
	Unread    bool
	Limit     int
	Cursor    string
}

// boundaryZonePolygon mirrors boundaryPolygon (boundarypage.go) at the exact
// parameter position zonepage.go's radius occupies ($3): every fast-path
// query below keeps the identical shape (and parameter count) of its circle
// counterpart in zonepage.go, with ST_Covers against the polygon replacing
// ST_DWithin against the circle.
const boundaryZonePolygon = "ST_GeomFromGeoJSON($3)::geography"

// boundaryDateFromWhere mirrors dateFromWhere: same projection (dateColumns),
// same $1/$2 centroid (via nearbyPoint), same $4 limit slot -- ST_Covers
// against the polygon ($3) replacing ST_DWithin against the circle ($3
// radius).
const boundaryDateFromWhere = "SELECT " + dateColumns +
	" FROM applications WHERE ST_Covers(" + boundaryZonePolygon + ", location)"

// First-page date-sort queries: polygon predicate only, ordered, limited.
const (
	boundaryNewestFirstQuery = boundaryDateFromWhere + newestOrderBy
	boundaryOldestFirstQuery = boundaryDateFromWhere + oldestOrderBy
)

// Keyset date-sort queries resuming after a cursor on a NON-NULL start_date
// row. Mirrors newestKeysetQuery/oldestKeysetQuery exactly. $5 the cursor's
// start_date (::date), $6 authority_code, $7 planit_name.
const (
	boundaryNewestKeysetQuery = boundaryDateFromWhere +
		" AND (start_date < $5::date OR start_date IS NULL" +
		" OR (start_date = $5::date AND (authority_code, planit_name) > ($6, $7)))" +
		newestOrderBy
	boundaryOldestKeysetQuery = boundaryDateFromWhere +
		" AND (start_date > $5::date OR start_date IS NULL" +
		" OR (start_date = $5::date AND (authority_code, planit_name) > ($6, $7)))" +
		oldestOrderBy
)

// Keyset date-sort queries resuming after a cursor in the NULL-start_date
// tail. Mirrors newestKeysetNullQuery/oldestKeysetNullQuery. $5 authority_code,
// $6 planit_name.
const (
	boundaryNewestKeysetNullQuery = boundaryDateFromWhere +
		" AND start_date IS NULL AND (authority_code, planit_name) > ($5, $6)" +
		newestOrderBy
	boundaryOldestKeysetNullQuery = boundaryDateFromWhere +
		" AND start_date IS NULL AND (authority_code, planit_name) > ($5, $6)" +
		oldestOrderBy
)

// boundaryStatusFirstQuery is the status first page: polygon predicate only,
// ordered, limited. It reuses boundaryDateFromWhere (same projection:
// appColumns + authority_code), mirroring statusFirstQuery.
const boundaryStatusFirstQuery = boundaryDateFromWhere + statusOrderBy

// Status keyset queries. Mirror the four statusKeyset* constants exactly
// (see their doc comments for the mixed-direction rationale), with the
// polygon predicate replacing the radius.
const (
	// $5 app_state, $6 start_date(::date), $7 authority_code, $8 planit_name.
	boundaryStatusKeysetQuery = boundaryDateFromWhere +
		" AND (app_state > $5 OR app_state IS NULL" +
		" OR (app_state = $5 AND (start_date < $6::date OR start_date IS NULL))" +
		" OR (app_state = $5 AND start_date = $6::date AND (authority_code, planit_name) > ($7, $8)))" +
		statusOrderBy
	// $5 app_state, $6 authority_code, $7 planit_name.
	boundaryStatusKeysetDateNullQuery = boundaryDateFromWhere +
		" AND (app_state > $5 OR app_state IS NULL" +
		" OR (app_state = $5 AND start_date IS NULL AND (authority_code, planit_name) > ($6, $7)))" +
		statusOrderBy
	// $5 start_date(::date), $6 authority_code, $7 planit_name.
	boundaryStatusKeysetStateNullQuery = boundaryDateFromWhere +
		" AND app_state IS NULL AND ((start_date < $5::date OR start_date IS NULL)" +
		" OR (start_date = $5::date AND (authority_code, planit_name) > ($6, $7)))" +
		statusOrderBy
	// $5 authority_code, $6 planit_name.
	boundaryStatusKeysetBothNullQuery = boundaryDateFromWhere +
		" AND app_state IS NULL AND start_date IS NULL AND (authority_code, planit_name) > ($5, $6)" +
		statusOrderBy
)

// boundaryActivityFromWhere mirrors activityFromWhere: same projection
// (dateColumns + activity_ts), same LEFT JOIN to the caller's unread
// notifications ($5 userID), same $1/$2 centroid and $4 limit slot --
// ST_Covers against the polygon ($3) replacing ST_DWithin against the circle.
const boundaryActivityFromWhere = "SELECT " + dateColumns + ", " + activityExpr + " AS activity_ts" +
	" FROM applications" +
	" LEFT JOIN (" + activityUnreadSubquery + ") u" +
	" ON applications.uid = u.application_uid AND applications.area_id = u.authority_id" +
	" WHERE ST_Covers(" + boundaryZonePolygon + ", location)"

// boundaryRecentActivityFirstQuery is the recent-activity first page: join +
// polygon predicate, ordered, limited.
const boundaryRecentActivityFirstQuery = boundaryActivityFromWhere + activityOrderBy

// Keyset recent-activity queries. Mirror recentActivityKeysetQuery /
// recentActivityKeysetNullQuery exactly.
const (
	// $6 the cursor's activity_ts (::timestamptz), $7 authority_code, $8 planit_name.
	boundaryRecentActivityKeysetQuery = boundaryActivityFromWhere +
		" AND (" + activityExpr + " < $6::timestamptz OR " + activityExpr + " IS NULL" +
		" OR (" + activityExpr + " = $6::timestamptz AND (authority_code, planit_name) > ($7, $8)))" +
		activityOrderBy
	// $6 authority_code, $7 planit_name.
	boundaryRecentActivityKeysetNullQuery = boundaryActivityFromWhere +
		" AND " + activityExpr + " IS NULL AND (authority_code, planit_name) > ($6, $7)" +
		activityOrderBy
)

// FindInBoundaryZonePage is FindInZonePage's custom-shape counterpart
// (GH#1031, tc-acbsh): one page of up to q.Limit applications ST_Covers-
// contained in q.Boundary, ordered by q.Sort and filtered by q.Status/
// q.Unread, plus an opaque sort-and-filter-aware cursor for the next page
// (empty when exhausted). It supports the same {distance, newest, oldest,
// status, recent-activity} sorts and status/unread filters as FindInZonePage,
// with ST_Covers against the polygon replacing ST_DWithin against the circle
// -- the boundary-aware equivalent findZonePage routes to as soon as a sort
// or filter is requested for a custom-shape zone (see
// watchzones/nearby.go's findZonePage). The cursor semantics are identical to
// FindInZonePage's: a sort mismatch returns ErrCursorSortMismatch, a filter
// mismatch ErrCursorFilterMismatch, a malformed token ErrCursorInvalid.
//
// q.UserID scopes the per-user notification data the recent-activity sort
// joins and the unread filter restricts on, exactly as InZoneQuery.UserID
// does.
func (s *PostgresStore) FindInBoundaryZonePage(ctx context.Context, q InBoundaryQuery) ([]PlanningApplication, string, error) {
	if !q.Sort.Supported() {
		return nil, "", fmt.Errorf("sort %q: %w", q.Sort, ErrUnsupportedSort)
	}
	var c *pageCursor
	if q.Cursor != "" {
		decoded, err := decodePageCursor(q.Cursor)
		if err != nil {
			return nil, "", err
		}
		if decoded.M != q.Sort {
			return nil, "", fmt.Errorf("cursor sort %q, request sort %q: %w", decoded.M, q.Sort, ErrCursorSortMismatch)
		}
		if decoded.F != q.Status || decoded.U != q.Unread {
			return nil, "", fmt.Errorf("cursor filter {status:%q unread:%t}, request filter {status:%q unread:%t}: %w",
				decoded.F, decoded.U, q.Status, q.Unread, ErrCursorFilterMismatch)
		}
		c = &decoded
	}

	boundaryJSON, err := encodeBoundaryGeoJSON(q.Boundary)
	if err != nil {
		return nil, "", fmt.Errorf("encode boundary geojson: %w", err)
	}

	// Filtered requests (a status value and/or the unread flag) take the
	// dynamic builder, composing the status predicate and/or the unread INNER
	// JOIN into the sort's query -- exactly as findFilteredZonePage does.
	// Unfiltered requests stay on the proven per-sort constant queries above.
	if q.Status != "" || q.Unread {
		return s.findFilteredBoundaryZonePage(ctx, q, boundaryJSON, c)
	}
	if q.Sort == SortDistance {
		return s.findDistanceBoundaryZonePage(ctx, q.Latitude, q.Longitude, boundaryJSON, q.Limit, c)
	}
	if q.Sort == SortStatus {
		return s.findStatusBoundaryZonePage(ctx, q.Latitude, q.Longitude, boundaryJSON, q.Limit, c)
	}
	if q.Sort == SortRecentActivity {
		return s.findRecentActivityBoundaryZonePage(ctx, q.UserID, q.Latitude, q.Longitude, boundaryJSON, q.Limit, c)
	}
	return s.findDateBoundaryZonePage(ctx, q.Latitude, q.Longitude, boundaryJSON, q.Sort, q.Limit, c)
}

// findDistanceBoundaryZonePage serves SortDistance: the same nearest-centroid
// KNN query FindInBoundaryPage uses (findBoundaryFirstPageQuery /
// findBoundaryKeysetQuery, boundarypage.go), but with a sort-aware
// (mode="distance") cursor so it composes with FindInBoundaryZonePage's
// shared sort/filter cursor contract -- mirroring zonepage.go's
// findDistanceZonePage.
func (s *PostgresStore) findDistanceBoundaryZonePage(ctx context.Context, latitude, longitude float64, boundaryJSON string, limit int, c *pageCursor) ([]PlanningApplication, string, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if c == nil {
		rows, err = s.db.Query(ctx, findBoundaryFirstPageQuery, longitude, latitude, boundaryJSON, limit)
	} else {
		dist, perr := strconv.ParseFloat(c.D, 64)
		if perr != nil {
			return nil, "", fmt.Errorf("parse distance cursor: %w: %w", ErrCursorInvalid, perr)
		}
		rows, err = s.db.Query(ctx, findBoundaryKeysetQuery, longitude, latitude, boundaryJSON, limit, dist, c.N)
	}
	wrap := func(err error) error {
		return fmt.Errorf("find applications by %s near (%v, %v): %w", SortDistance, latitude, longitude, err)
	}
	if err != nil {
		return nil, "", wrap(err)
	}
	return collectPage(rows, scanNearbyRow, nearbyAppOf,
		func(last nearbyRow) (string, error) {
			enc, err := encodePageCursor(pageCursor{
				M: SortDistance,
				D: strconv.FormatFloat(last.dist, 'g', -1, 64),
				N: last.app.Name,
			})
			if err != nil {
				return "", fmt.Errorf("encode distance cursor: %w", err)
			}
			return enc, nil
		},
		limit, wrap)
}

// findDateBoundaryZonePage serves SortNewest/SortOldest, mirroring
// zonepage.go's findDateZonePage: it reuses collectKeysetPage and
// dateCursorOf unchanged, only the query selection (boundaryDateZoneQuery)
// differs.
func (s *PostgresStore) findDateBoundaryZonePage(ctx context.Context, latitude, longitude float64, boundaryJSON string, sort Sort, limit int, c *pageCursor) ([]PlanningApplication, string, error) {
	query, args := boundaryDateZoneQuery(sort, latitude, longitude, boundaryJSON, limit, c)
	return s.collectKeysetPage(ctx, sort, latitude, longitude, query, args, limit, func(last datePageRow) pageCursor {
		return dateCursorOf(sort, last)
	})
}

// boundaryDateZoneQuery mirrors dateZoneQuery: the base args ($1..$4) are
// longitude, latitude, the boundary GeoJSON, limit; the keyset args extend
// them exactly as dateZoneQuery's do.
func boundaryDateZoneQuery(sort Sort, latitude, longitude float64, boundaryJSON string, limit int, c *pageCursor) (string, []any) {
	base := []any{longitude, latitude, boundaryJSON, limit}
	switch {
	case c == nil:
		if sort == SortNewest {
			return boundaryNewestFirstQuery, base
		}
		return boundaryOldestFirstQuery, base
	case c.SD == nil:
		args := append(base, c.AC, c.N)
		if sort == SortNewest {
			return boundaryNewestKeysetNullQuery, args
		}
		return boundaryOldestKeysetNullQuery, args
	default:
		args := append(base, *c.SD, c.AC, c.N)
		if sort == SortNewest {
			return boundaryNewestKeysetQuery, args
		}
		return boundaryOldestKeysetQuery, args
	}
}

// findStatusBoundaryZonePage serves SortStatus, mirroring zonepage.go's
// findStatusZonePage: it reuses collectKeysetPage and statusCursorOf
// unchanged, only the query selection (boundaryStatusZoneQuery) differs.
func (s *PostgresStore) findStatusBoundaryZonePage(ctx context.Context, latitude, longitude float64, boundaryJSON string, limit int, c *pageCursor) ([]PlanningApplication, string, error) {
	query, args := boundaryStatusZoneQuery(latitude, longitude, boundaryJSON, limit, c)
	return s.collectKeysetPage(ctx, SortStatus, latitude, longitude, query, args, limit, statusCursorOf)
}

// boundaryStatusZoneQuery mirrors statusZoneQuery: the base args ($1..$4) are
// longitude, latitude, the boundary GeoJSON, limit; the keyset args extend
// them exactly as statusZoneQuery's do.
func boundaryStatusZoneQuery(latitude, longitude float64, boundaryJSON string, limit int, c *pageCursor) (string, []any) {
	base := []any{longitude, latitude, boundaryJSON, limit}
	switch {
	case c == nil:
		return boundaryStatusFirstQuery, base
	case c.AS != nil && c.SD != nil:
		return boundaryStatusKeysetQuery, append(base, *c.AS, *c.SD, c.AC, c.N)
	case c.AS != nil: // start_date NULL: cursor in its group's NULL-start_date tail.
		return boundaryStatusKeysetDateNullQuery, append(base, *c.AS, c.AC, c.N)
	case c.SD != nil: // app_state NULL: cursor in the NULL-app_state tail.
		return boundaryStatusKeysetStateNullQuery, append(base, *c.SD, c.AC, c.N)
	default: // both NULL: the deepest tail.
		return boundaryStatusKeysetBothNullQuery, append(base, c.AC, c.N)
	}
}

// findRecentActivityBoundaryZonePage serves SortRecentActivity, mirroring
// zonepage.go's findRecentActivityZonePage: it reuses scanActivityPageRow /
// activityAppOf / activityCursorOf unchanged, only the query selection
// (boundaryRecentActivityZoneQuery) differs. userID scopes the joined unread
// notifications.
func (s *PostgresStore) findRecentActivityBoundaryZonePage(ctx context.Context, userID string, latitude, longitude float64, boundaryJSON string, limit int, c *pageCursor) ([]PlanningApplication, string, error) {
	query, args := boundaryRecentActivityZoneQuery(userID, latitude, longitude, boundaryJSON, limit, c)
	wrap := func(err error) error {
		return fmt.Errorf("find applications by %s near (%v, %v): %w", SortRecentActivity, latitude, longitude, err)
	}
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, "", wrap(err)
	}
	return collectPage(rows, scanActivityPageRow, activityAppOf,
		func(last activityPageRow) (string, error) {
			enc, err := encodePageCursor(activityCursorOf(last))
			if err != nil {
				return "", fmt.Errorf("encode %s cursor: %w", SortRecentActivity, err)
			}
			return enc, nil
		},
		limit, wrap)
}

// boundaryRecentActivityZoneQuery mirrors recentActivityZoneQuery: the base
// args ($1..$5) are longitude, latitude, the boundary GeoJSON, limit, userID;
// the keyset args extend them exactly as recentActivityZoneQuery's do.
func boundaryRecentActivityZoneQuery(userID string, latitude, longitude float64, boundaryJSON string, limit int, c *pageCursor) (string, []any) {
	base := []any{longitude, latitude, boundaryJSON, limit, userID}
	switch {
	case c == nil:
		return boundaryRecentActivityFirstQuery, base
	case c.TS == nil:
		return boundaryRecentActivityKeysetNullQuery, append(base, c.AC, c.N)
	default:
		return boundaryRecentActivityKeysetQuery, append(base, *c.TS, c.AC, c.N)
	}
}

// findFilteredBoundaryZonePage serves the status/unread filtered requests,
// mirroring zonepage.go's findFilteredZonePage: it composes the same per-sort
// ORDER BY + keyset predicates (via the shared orderByExpr/keysetPredicate)
// with a status predicate and/or an unread join, then collects with the
// sort's projection scanner and mints a filter-stamped next cursor. c is the
// validated (sort- and filter-matched) cursor, nil on the first page.
func (s *PostgresStore) findFilteredBoundaryZonePage(ctx context.Context, q InBoundaryQuery, boundaryJSON string, c *pageCursor) ([]PlanningApplication, string, error) {
	query, args, err := buildFilteredBoundaryZoneQuery(q, boundaryJSON, c)
	if err != nil {
		return nil, "", err
	}
	switch q.Sort {
	case SortDistance:
		return s.collectFilteredBoundaryDistancePage(ctx, q, query, args)
	case SortRecentActivity:
		return s.collectFilteredBoundaryActivityPage(ctx, q, query, args)
	default: // newest, oldest, status — the date-projection sorts
		cursorOf := func(last datePageRow) pageCursor {
			var cur pageCursor
			if q.Sort == SortStatus {
				cur = statusCursorOf(last)
			} else {
				cur = dateCursorOf(q.Sort, last)
			}
			cur.F, cur.U = q.Status, q.Unread
			return cur
		}
		return s.collectKeysetPage(ctx, q.Sort, q.Latitude, q.Longitude, query, args, q.Limit, cursorOf)
	}
}

// buildFilteredBoundaryZoneQuery mirrors buildFilteredZoneQuery: the shape --
// projection, optional unread join, optional status predicate, keyset
// predicate, ORDER BY, LIMIT -- is byte-identical, built via the exact same
// shared orderByExpr/keysetPredicate helpers. Only the spatial predicate
// differs: ST_Covers against the polygon (boundaryJSON) replacing ST_DWithin
// against the circle (q.RadiusMetres).
func buildFilteredBoundaryZoneQuery(q InBoundaryQuery, boundaryJSON string, c *pageCursor) (string, []any, error) {
	b := &argBuilder{}
	lonP := b.add(q.Longitude)
	latP := b.add(q.Latitude)
	point := "ST_SetSRID(ST_MakePoint(" + lonP + ", " + latP + "), 4326)::geography"

	var projection string
	switch q.Sort {
	case SortDistance:
		projection = appColumns + ", location <-> " + point + " AS distance"
	case SortRecentActivity:
		projection = dateColumns + ", " + activityExpr + " AS activity_ts"
	default:
		projection = dateColumns
	}

	var sb strings.Builder
	sb.WriteString("SELECT ")
	sb.WriteString(projection)
	sb.WriteString(" FROM applications")

	if q.Unread || q.Sort == SortRecentActivity {
		joinType := "LEFT JOIN"
		if q.Unread {
			joinType = "JOIN" // INNER: drop applications with no unread for the caller.
		}
		userP := b.add(q.UserID)
		subquery := "SELECT n.application_uid, n.authority_id, MAX(n.created_at) AS created_at" +
			" FROM notifications n" +
			" WHERE n.user_id = " + userP + " AND n.read_at IS NULL" +
			" GROUP BY n.application_uid, n.authority_id"
		sb.WriteString(" " + joinType + " (" + subquery + ") u" +
			" ON applications.uid = u.application_uid AND applications.area_id = u.authority_id")
	}

	polygonP := b.add(boundaryJSON)
	sb.WriteString(" WHERE ST_Covers(ST_GeomFromGeoJSON(" + polygonP + ")::geography, location)")

	if q.Status != "" {
		sb.WriteString(" AND app_state = " + b.add(q.Status))
	}

	if c != nil {
		predicate, err := keysetPredicate(q.Sort, point, c, b)
		if err != nil {
			return "", nil, err
		}
		sb.WriteString(" AND " + predicate)
	}

	sb.WriteString(" ORDER BY " + orderByExpr(q.Sort, point))
	sb.WriteString(" LIMIT " + b.add(q.Limit))

	return sb.String(), b.args, nil
}

// collectFilteredBoundaryDistancePage mirrors collectFilteredDistancePage: a
// filtered distance query (nearbyRow projection) minting a filter-stamped
// (distance, planit_name) next cursor.
func (s *PostgresStore) collectFilteredBoundaryDistancePage(ctx context.Context, q InBoundaryQuery, query string, args []any) ([]PlanningApplication, string, error) {
	wrap := func(err error) error {
		return fmt.Errorf("find filtered applications by distance near (%v, %v): %w", q.Latitude, q.Longitude, err)
	}
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, "", wrap(err)
	}
	return collectPage(rows, scanNearbyRow, nearbyAppOf,
		func(last nearbyRow) (string, error) {
			enc, err := encodePageCursor(pageCursor{
				M: SortDistance, F: q.Status, U: q.Unread,
				D: strconv.FormatFloat(last.dist, 'g', -1, 64), N: last.app.Name,
			})
			if err != nil {
				return "", fmt.Errorf("encode distance cursor: %w", err)
			}
			return enc, nil
		},
		q.Limit, wrap)
}

// collectFilteredBoundaryActivityPage mirrors collectFilteredActivityPage: a
// filtered recent-activity query (activity projection) minting a
// filter-stamped (activity_ts, authority_code, planit_name) next cursor.
func (s *PostgresStore) collectFilteredBoundaryActivityPage(ctx context.Context, q InBoundaryQuery, query string, args []any) ([]PlanningApplication, string, error) {
	wrap := func(err error) error {
		return fmt.Errorf("find filtered applications by %s near (%v, %v): %w", SortRecentActivity, q.Latitude, q.Longitude, err)
	}
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, "", wrap(err)
	}
	return collectPage(rows, scanActivityPageRow, activityAppOf,
		func(last activityPageRow) (string, error) {
			cur := activityCursorOf(last)
			cur.F, cur.U = q.Status, q.Unread
			enc, err := encodePageCursor(cur)
			if err != nil {
				return "", fmt.Errorf("encode %s cursor: %w", SortRecentActivity, err)
			}
			return enc, nil
		},
		q.Limit, wrap)
}
