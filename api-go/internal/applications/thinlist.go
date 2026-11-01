package applications

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const thinTable = "application_locations"

// thinListSwitch reports whether the paged list reads use application_locations.
type thinListSwitch interface {
	ThinList(ctx context.Context) bool
}

// WithReadPath sets the switch the paged list reads consult and returns the
// store. Without one every read stays on applications.
func (s *PostgresStore) WithReadPath(sw thinListSwitch) *PostgresStore {
	s.readPath = sw
	return s
}

func (s *PostgresStore) thinList(ctx context.Context) bool {
	return s.readPath != nil && s.readPath.ThinList(ctx)
}

type appKey struct {
	authorityCode string
	name          string
}

// thinRow is one find result: the key plus the sort values a cursor needs. Only
// the Name, StartDate and AppState fields of app are set.
type thinRow struct {
	key      appKey
	app      PlanningApplication
	dist     float64
	activity *time.Time
}

func scanThinDistance(row pgx.CollectableRow) (thinRow, error) {
	var r thinRow
	err := row.Scan(&r.key.authorityCode, &r.key.name, &r.dist)
	r.app.Name = r.key.name
	return r, err
}

func scanThinDate(row pgx.CollectableRow) (thinRow, error) {
	var r thinRow
	err := row.Scan(&r.key.authorityCode, &r.key.name, &r.app.StartDate)
	r.app.Name = r.key.name
	return r, err
}

func scanThinStatus(row pgx.CollectableRow) (thinRow, error) {
	var r thinRow
	err := row.Scan(&r.key.authorityCode, &r.key.name, &r.app.AppState, &r.app.StartDate)
	r.app.Name = r.key.name
	return r, err
}

func scanThinActivity(row pgx.CollectableRow) (thinRow, error) {
	var r thinRow
	err := row.Scan(&r.key.authorityCode, &r.key.name, &r.activity)
	r.app.Name = r.key.name
	return r, err
}

func thinScanner(sort Sort) func(pgx.CollectableRow) (thinRow, error) {
	switch sort {
	case SortDistance:
		return scanThinDistance
	case SortStatus:
		return scanThinStatus
	case SortRecentActivity:
		return scanThinActivity
	default:
		return scanThinDate
	}
}

// thinCursor builds the next-page cursor from the last find row, in the format
// the full read mints, so a cursor is valid on either side of the switch.
func thinCursor(sort Sort, status string, unread bool, r thinRow) (string, error) {
	var cur pageCursor
	ac := r.key.authorityCode
	switch sort {
	case SortDistance:
		cur = pageCursor{M: SortDistance, D: strconv.FormatFloat(r.dist, 'g', -1, 64), N: r.key.name}
	case SortStatus:
		cur = statusCursorOf(datePageRow{app: r.app, ac: ac})
	case SortRecentActivity:
		cur = activityCursorOf(activityPageRow{app: r.app, ac: ac, activity: r.activity})
	default:
		cur = dateCursorOf(sort, datePageRow{app: r.app, ac: ac})
	}
	cur.F, cur.U = status, unread
	enc, err := encodePageCursor(cur)
	if err != nil {
		return "", fmt.Errorf("encode %s cursor: %w", sort, err)
	}
	return enc, nil
}

const thinDistanceColumns = "authority_code, planit_name"

// thinDateColumns and thinStatusColumns are the find projections that replace
// dateColumns in the constant date-sort and status queries.
const (
	thinDateColumns   = "authority_code, planit_name, start_date"
	thinStatusColumns = "authority_code, planit_name, app_state, start_date"
)

func thinReplacer(columns string) *strings.Replacer {
	return strings.NewReplacer(
		dateColumns, columns,
		" FROM applications", " FROM "+thinTable,
		"applications.uid", thinTable+".uid",
		"applications.area_id", thinTable+".area_id",
	)
}

var (
	thinDateReplacer     = thinReplacer(thinDateColumns)
	thinStatusReplacer   = thinReplacer(thinStatusColumns)
	thinActivityReplacer = thinReplacer(thinDistanceColumns)
)

// thinQuery rewrites a constant list query for sort onto application_locations,
// keeping its predicate, order and keyset text as they are.
func thinQuery(sort Sort, query string) string {
	switch sort {
	case SortStatus:
		return thinStatusReplacer.Replace(query)
	case SortRecentActivity:
		return thinActivityReplacer.Replace(query)
	default:
		return thinDateReplacer.Replace(query)
	}
}

// nearbyQueries returns the first-page and keyset nearest-first queries over
// table, selecting columns and the distance.
func nearbyQueries(columns, table string) (first, keyset string) {
	first = knnThenExactOn(columns, table, "ST_DWithin(location, "+nearbyPoint+", $3)", "$4")
	keyset = knnThenExactOn(columns, table, "ST_DWithin(location, "+nearbyPoint+", $3) "+
		"AND (location <-> "+nearbyPoint+" > $5 "+
		"OR (location <-> "+nearbyPoint+" = $5 AND planit_name > $6))", "$4")
	return first, keyset
}

func boundaryQueries(columns, table string) (first, keyset string) {
	first = knnThenExactOn(columns, table, "ST_Covers("+boundaryPolygon+", location)", "$4")
	keyset = knnThenExactOn(columns, table, "ST_Covers("+boundaryPolygon+", location) "+
		"AND (location <-> "+nearbyPoint+" > $5 "+
		"OR (location <-> "+nearbyPoint+" = $5 AND planit_name > $6))", "$4")
	return first, keyset
}

var (
	thinNearbyFirstQuery, thinNearbyKeysetQuery     = nearbyQueries(thinDistanceColumns, thinTable)
	thinBoundaryFirstQuery, thinBoundaryKeysetQuery = boundaryQueries(thinDistanceColumns, thinTable)
)

// thinBoundaryStatusDistanceQuery is the nearest-first boundary query for a
// status filter, routed through knnThenExact like the unfiltered one.
func thinBoundaryStatusDistanceQuery(q InBoundaryQuery, boundaryJSON string, c *pageCursor) (string, []any, error) {
	b := &argBuilder{}
	b.add(q.Longitude)
	b.add(q.Latitude)
	b.add(boundaryJSON)
	limitP := b.add(q.Limit)
	filter := "ST_Covers(" + boundaryPolygon + ", location) AND app_state = " + b.add(q.Status)
	if c != nil {
		dist, err := strconv.ParseFloat(c.D, 64)
		if err != nil {
			return "", nil, fmt.Errorf("parse distance cursor: %w: %w", ErrCursorInvalid, err)
		}
		distP := b.add(dist)
		nameP := b.add(c.N)
		filter += " AND (location <-> " + nearbyPoint + " > " + distP +
			" OR (location <-> " + nearbyPoint + " = " + distP + " AND planit_name > " + nameP + "))"
	}
	return knnThenExactOn(thinDistanceColumns, thinTable, filter, limitP), b.args, nil
}

// findRunner runs fn in a read-only transaction with a planner setting applied.
type findRunner func(ctx context.Context, fn func(pgx.Tx) error) error

func (s *PostgresStore) noNestLoopIf(on bool) findRunner {
	if on {
		return s.withNoNestLoop
	}
	return nil
}

func (s *PostgresStore) findThinRows(ctx context.Context, run findRunner, query string, args []any, scan func(pgx.CollectableRow) (thinRow, error)) ([]thinRow, error) {
	if run == nil {
		rows, err := s.db.Query(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		return pgx.CollectRows(rows, scan)
	}
	var found []thinRow
	err := run(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, query, append([]any{pgx.QueryExecModeExec}, args...)...)
		if err != nil {
			return err
		}
		found, err = pgx.CollectRows(rows, scan)
		return err
	})
	return found, err
}

const hydrateQuery = "SELECT a.authority_code, " + appColumns +
	" FROM unnest($1::text[], $2::text[]) AS k(ac, nm)" +
	" JOIN applications a ON a.authority_code = k.ac AND a.planit_name = k.nm"

// hydrate reads the full rows for found by primary key and returns them in the
// order of found. A key with no row is skipped.
func (s *PostgresStore) hydrate(ctx context.Context, found []thinRow) ([]PlanningApplication, error) {
	apps := make([]PlanningApplication, 0, len(found))
	if len(found) == 0 {
		return apps, nil
	}
	codes := make([]string, len(found))
	names := make([]string, len(found))
	for i, r := range found {
		codes[i], names[i] = r.key.authorityCode, r.key.name
	}
	rows, err := s.db.Query(ctx, hydrateQuery, codes, names)
	if err != nil {
		return nil, err
	}
	type hydrated struct {
		key appKey
		app PlanningApplication
	}
	got, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (hydrated, error) {
		var h hydrated
		err := row.Scan(append([]any{&h.key.authorityCode}, appScanDest(&h.app)...)...)
		h.key.name = h.app.Name
		return h, err
	})
	if err != nil {
		return nil, err
	}
	byKey := make(map[appKey]PlanningApplication, len(got))
	for _, h := range got {
		byKey[h.key] = h.app
	}
	for _, r := range found {
		if a, ok := byKey[r.key]; ok {
			apps = append(apps, a)
		}
	}
	return apps, nil
}

// pageThin finds the page's keys on application_locations, mints the next cursor
// from the last found row, then hydrates the rows from applications.
func (s *PostgresStore) pageThin(ctx context.Context, run findRunner, query string, args []any, sort Sort, limit int, cursorOf func(thinRow) (string, error), wrap func(error) error) ([]PlanningApplication, string, error) {
	found, err := s.findThinRows(ctx, run, query, args, thinScanner(sort))
	if err != nil {
		return nil, "", wrap(err)
	}
	next := ""
	if limit > 0 && len(found) == limit {
		if next, err = cursorOf(found[len(found)-1]); err != nil {
			return nil, "", err
		}
	}
	apps, err := s.hydrate(ctx, found)
	if err != nil {
		return nil, "", wrap(err)
	}
	return apps, next, nil
}

func (s *PostgresStore) findZoneThin(ctx context.Context, q InZoneQuery, c *pageCursor) ([]PlanningApplication, string, error) {
	wrap := func(err error) error {
		return fmt.Errorf("find applications by %s near (%v, %v): %w", q.Sort, q.Latitude, q.Longitude, err)
	}
	var (
		query string
		args  []any
		run   findRunner
	)
	activityJoin := q.Unread || q.Sort == SortRecentActivity
	switch {
	case q.Status != "" || q.Unread:
		var err error
		if query, args, err = buildThinFilteredZoneQuery(q, c); err != nil {
			return nil, "", err
		}
		run = s.noNestLoopIf(activityJoin)
	case q.Sort == SortDistance:
		query, args = thinNearbyFirstQuery, []any{q.Longitude, q.Latitude, q.RadiusMetres, q.Limit}
		if c != nil {
			dist, err := strconv.ParseFloat(c.D, 64)
			if err != nil {
				return nil, "", fmt.Errorf("parse distance cursor: %w: %w", ErrCursorInvalid, err)
			}
			query, args = thinNearbyKeysetQuery, append(args, dist, c.N)
		}
		run = s.withKNNWalk
	case q.Sort == SortStatus:
		query, args = statusZoneQuery(q.Latitude, q.Longitude, q.RadiusMetres, q.Limit, c)
		query = thinQuery(q.Sort, query)
	case q.Sort == SortRecentActivity:
		query, args = recentActivityZoneQuery(q.UserID, q.Latitude, q.Longitude, q.RadiusMetres, q.Limit, c)
		query, run = thinQuery(q.Sort, query), s.withNoNestLoop
	default:
		query, args = dateZoneQuery(q.Sort, q.Latitude, q.Longitude, q.RadiusMetres, q.Limit, c)
		query = thinQuery(q.Sort, query)
	}
	return s.pageThin(ctx, run, query, args, q.Sort, q.Limit, func(r thinRow) (string, error) {
		return thinCursor(q.Sort, q.Status, q.Unread, r)
	}, wrap)
}

func (s *PostgresStore) findBoundaryZoneThin(ctx context.Context, q InBoundaryQuery, boundaryJSON string, c *pageCursor) ([]PlanningApplication, string, error) {
	wrap := func(err error) error {
		return fmt.Errorf("find applications by %s near (%v, %v): %w", q.Sort, q.Latitude, q.Longitude, err)
	}
	var (
		query string
		args  []any
		run   findRunner
		err   error
	)
	switch {
	case q.Sort == SortDistance && q.Status != "" && !q.Unread:
		if query, args, err = thinBoundaryStatusDistanceQuery(q, boundaryJSON, c); err != nil {
			return nil, "", err
		}
		run = s.withKNNWalk
	case q.Status != "" || q.Unread:
		if query, args, err = buildThinFilteredBoundaryZoneQuery(q, boundaryJSON, c); err != nil {
			return nil, "", err
		}
		run = s.noNestLoopIf(q.Unread || q.Sort == SortRecentActivity)
	case q.Sort == SortDistance:
		query, args = thinBoundaryFirstQuery, []any{q.Longitude, q.Latitude, boundaryJSON, q.Limit}
		if c != nil {
			dist, perr := strconv.ParseFloat(c.D, 64)
			if perr != nil {
				return nil, "", fmt.Errorf("parse distance cursor: %w: %w", ErrCursorInvalid, perr)
			}
			query, args = thinBoundaryKeysetQuery, append(args, dist, c.N)
		}
		run = s.withKNNWalk
	case q.Sort == SortStatus:
		query, args = boundaryStatusZoneQuery(boundaryJSON, q.Limit, c)
		query = thinQuery(q.Sort, query)
	case q.Sort == SortRecentActivity:
		query, args = boundaryRecentActivityZoneQuery(q.UserID, boundaryJSON, q.Limit, c)
		query, run = thinQuery(q.Sort, query), s.withNoNestLoop
	default:
		query, args = boundaryDateZoneQuery(q.Sort, boundaryJSON, q.Limit, c)
		query = thinQuery(q.Sort, query)
	}
	return s.pageThin(ctx, run, query, args, q.Sort, q.Limit, func(r thinRow) (string, error) {
		return thinCursor(q.Sort, q.Status, q.Unread, r)
	}, wrap)
}
