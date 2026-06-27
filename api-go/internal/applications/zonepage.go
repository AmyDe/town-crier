package applications

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
)

// Sort is a server-side ordering for the in-zone applications page. It is part of
// the API contract: a request's ?sort= value and the mode embedded in a keyset
// cursor are both this type, so a cursor minted under one sort can be rejected
// when replayed under another (see FindInZonePage and ErrCursorSortMismatch).
//
// Slices 1-3 (epic #682) implement {distance, newest, oldest, status,
// recent-activity} — the full UI sort set.
type Sort string

const (
	// SortDistance orders nearest-first via the KNN <-> operator. It is the
	// default and the cheapest plan (GiST-served), preserving the legacy
	// nearest-first browse behaviour.
	SortDistance Sort = "distance"
	// SortNewest orders by start_date DESC NULLS LAST, then the unique tiebreak
	// (authority_code, planit_name).
	SortNewest Sort = "newest"
	// SortOldest orders by start_date ASC NULLS LAST, then the unique tiebreak
	// (authority_code, planit_name). It reuses the newest btree, scanned backward.
	SortOldest Sort = "oldest"
	// SortStatus orders by app_state ASC NULLS LAST, then start_date DESC NULLS
	// LAST, then the unique tiebreak (authority_code, planit_name). The mixed
	// directions (app_state ASC / start_date DESC) make the keyset predicate
	// per-key (see findStatusZonePage).
	SortStatus Sort = "status"
	// SortRecentActivity orders by GREATEST(start_date::timestamptz, the caller's
	// latest unread notification for the application) DESC NULLS LAST, then the
	// unique tiebreak (authority_code, planit_name). It is per-user: the unread
	// timestamp comes from a LEFT JOIN of the caller's notifications, so an app
	// with a fresh unread floats above an app with a newer start_date but no
	// unread (see findRecentActivityZonePage).
	SortRecentActivity Sort = "recent-activity"
)

// Supported reports whether s is a server-side sort. The set is the five UI
// sorts; anything else is rejected so the handler returns 400 rather than
// running an arbitrary order.
func (s Sort) Supported() bool {
	switch s {
	case SortDistance, SortNewest, SortOldest, SortStatus, SortRecentActivity:
		return true
	default:
		return false
	}
}

var (
	// ErrCursorInvalid signals a ?cursor= token that is not a well-formed keyset
	// cursor (bad base64 or non-cursor payload). Consumers map it to HTTP 400.
	ErrCursorInvalid = errors.New("invalid page cursor")
	// ErrCursorSortMismatch signals a cursor whose embedded sort mode differs
	// from the request's sort. A keyset cursor is only valid for the ORDER BY it
	// was minted under, so replaying it under a different sort would yield a
	// mis-ordered page; consumers map this to HTTP 400, never a silent reset.
	ErrCursorSortMismatch = errors.New("cursor sort mode does not match request sort")
	// ErrUnsupportedSort signals a sort value outside the set this slice
	// implements. Consumers map it to HTTP 400.
	ErrUnsupportedSort = errors.New("unsupported sort")
)

// pageCursor is the opaque, sort-aware keyset position handed back to clients.
// It embeds the sort mode (M) so a stale cursor cannot silently produce a page
// in the wrong order, plus exactly the keys that sort's ORDER BY ranks on:
//   - distance: D (the KNN distance) + N (planit_name).
//   - newest/oldest: SD (start_date, nil for a NULL-start_date tail row) + AC
//     (authority_code) + N (planit_name).
//   - status: AS (app_state, nil for a NULL-app_state tail row) + SD (start_date,
//     nil for a NULL-start_date tail row) + AC (authority_code) + N (planit_name).
//   - recent-activity: TS (the activity timestamp, nil for a NULL-activity tail
//     row) + AC (authority_code) + N (planit_name).
//
// It is base64url-encoded JSON. AS, SD and TS use a nil pointer (omitted field)
// to mean a NULL key value, so the next page's predicate can pick the matching
// NULLS-LAST tail branch. SD is a "2006-01-02" date string so the predicate
// compares against an unambiguous ::date, not a timezone-laden timestamp; TS is
// an RFC3339Nano timestamp string compared against a ::timestamptz.
type pageCursor struct {
	M  Sort    `json:"m"`
	D  string  `json:"d,omitempty"`
	AS *string `json:"as,omitempty"`
	SD *string `json:"sd,omitempty"`
	TS *string `json:"ts,omitempty"`
	AC string  `json:"ac,omitempty"`
	N  string  `json:"n"`
}

// encodePageCursor serialises the keyset position to a base64url JSON token.
func encodePageCursor(c pageCursor) (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshal page cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// decodePageCursor parses a base64url JSON token back into a keyset position. A
// malformed token (bad base64 or non-cursor JSON) is reported as ErrCursorInvalid.
func decodePageCursor(token string) (pageCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return pageCursor{}, fmt.Errorf("base64 decode page cursor: %w: %w", ErrCursorInvalid, err)
	}
	var c pageCursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return pageCursor{}, fmt.Errorf("unmarshal page cursor: %w: %w", ErrCursorInvalid, err)
	}
	return c, nil
}

// dateColumns is the read projection for the start_date-ordered sorts: the shared
// appColumns plus authority_code, the second keyset key (a PlanIt name is only
// unique within an authority, so the tiebreak needs both). Its order MUST match
// scanDatePageRow.
const dateColumns = appColumns + ", authority_code"

// The ORDER BY + LIMIT suffix for each date sort. NULLS LAST keeps NULL-start_date
// rows after every dated row; (authority_code, planit_name) is the unique tiebreak.
const (
	newestOrderBy = " ORDER BY start_date DESC NULLS LAST, authority_code, planit_name LIMIT $4"
	oldestOrderBy = " ORDER BY start_date ASC NULLS LAST, authority_code, planit_name LIMIT $4"
)

// dateFromWhere is the shared SELECT + spatial predicate for the date sorts. $1
// is longitude and $2 latitude (via nearbyPoint), $3 the radius, $4 the limit.
const dateFromWhere = "SELECT " + dateColumns + " FROM applications WHERE ST_DWithin(location, " + nearbyPoint + ", $3)"

// First-page queries: spatial predicate only, ordered, limited.
const (
	newestFirstQuery = dateFromWhere + newestOrderBy
	oldestFirstQuery = dateFromWhere + oldestOrderBy
)

// Keyset queries resuming after a cursor on a NON-NULL start_date row. The
// predicate exactly matches the ORDER BY so pages never overlap or gap: rows
// further along the date order, OR all NULL-start_date rows (they trail in NULLS
// LAST), OR an equal-date row past the (authority_code, planit_name) tiebreak.
// $5 is the cursor's start_date (::date), $6 authority_code, $7 planit_name.
const (
	newestKeysetQuery = dateFromWhere +
		" AND (start_date < $5::date OR start_date IS NULL" +
		" OR (start_date = $5::date AND (authority_code, planit_name) > ($6, $7)))" +
		newestOrderBy
	oldestKeysetQuery = dateFromWhere +
		" AND (start_date > $5::date OR start_date IS NULL" +
		" OR (start_date = $5::date AND (authority_code, planit_name) > ($6, $7)))" +
		oldestOrderBy
)

// Keyset queries resuming after a cursor on a NULL start_date row — the NULLS
// LAST tail, where the only remaining order is the (authority_code, planit_name)
// tiebreak. $5 is authority_code, $6 planit_name.
const (
	newestKeysetNullQuery = dateFromWhere +
		" AND start_date IS NULL AND (authority_code, planit_name) > ($5, $6)" +
		newestOrderBy
	oldestKeysetNullQuery = dateFromWhere +
		" AND start_date IS NULL AND (authority_code, planit_name) > ($5, $6)" +
		oldestOrderBy
)

// statusOrderBy is the ORDER BY + LIMIT suffix for SortStatus. app_state ASC NULLS
// LAST groups by state with the NULL-app_state rows trailing; within a group
// start_date DESC NULLS LAST orders newest-first with NULL-start_date rows
// trailing; (authority_code, planit_name) is the unique tiebreak. The mixed
// directions (state ASC / date DESC) make the keyset predicate per-key.
const statusOrderBy = " ORDER BY app_state ASC NULLS LAST, start_date DESC NULLS LAST, authority_code, planit_name LIMIT $4"

// statusFirstQuery is the status first page: spatial predicate only, ordered,
// limited. It reuses dateFromWhere (same projection: appColumns + authority_code).
const statusFirstQuery = dateFromWhere + statusOrderBy

// Status keyset queries. The "strictly after the cursor" predicate exactly mirrors
// statusOrderBy as a lexicographic OR-chain, honouring each column's direction and
// NULLS LAST, so pages never overlap or gap:
//
//	after-col1 (app_state ASC NULLS LAST)
//	  OR (eq-col1 AND after-col2 (start_date DESC NULLS LAST))
//	  OR (eq-col1 AND eq-col2 AND (authority_code, planit_name) > tiebreak)
//
// Because app_state and start_date are each independently nullable, the cursor row
// sits in one of four NULLS-LAST positions, each needing its own predicate (a NULL
// key has no "strictly after" at that column — nothing follows a NULLS-LAST tail —
// and equality at a NULL key is `IS NULL`):
//
//	statusKeysetQuery          app_state non-null, start_date non-null
//	statusKeysetDateNullQuery  app_state non-null, start_date NULL
//	statusKeysetStateNullQuery app_state NULL,     start_date non-null
//	statusKeysetBothNullQuery  app_state NULL,     start_date NULL
const (
	// $5 app_state, $6 start_date(::date), $7 authority_code, $8 planit_name.
	statusKeysetQuery = dateFromWhere +
		" AND (app_state > $5 OR app_state IS NULL" +
		" OR (app_state = $5 AND (start_date < $6::date OR start_date IS NULL))" +
		" OR (app_state = $5 AND start_date = $6::date AND (authority_code, planit_name) > ($7, $8)))" +
		statusOrderBy
	// $5 app_state, $6 authority_code, $7 planit_name. Cursor in the NULL-start_date
	// tail of its app_state group: no start_date follows it, so equal-state only
	// advances on the tiebreak; a greater (or NULL) app_state still follows.
	statusKeysetDateNullQuery = dateFromWhere +
		" AND (app_state > $5 OR app_state IS NULL" +
		" OR (app_state = $5 AND start_date IS NULL AND (authority_code, planit_name) > ($6, $7)))" +
		statusOrderBy
	// $5 start_date(::date), $6 authority_code, $7 planit_name. Cursor in the
	// NULL-app_state tail: no app_state follows it, so the scan stays within
	// app_state IS NULL and advances on start_date DESC NULLS LAST then the tiebreak.
	statusKeysetStateNullQuery = dateFromWhere +
		" AND app_state IS NULL AND ((start_date < $5::date OR start_date IS NULL)" +
		" OR (start_date = $5::date AND (authority_code, planit_name) > ($6, $7)))" +
		statusOrderBy
	// $5 authority_code, $6 planit_name. Deepest tail: NULL app_state AND NULL
	// start_date — only the (authority_code, planit_name) tiebreak remains.
	statusKeysetBothNullQuery = dateFromWhere +
		" AND app_state IS NULL AND start_date IS NULL AND (authority_code, planit_name) > ($5, $6)" +
		statusOrderBy
)

// datePageRow carries a hydrated application plus its authority_code, so the last
// row of a full page can be encoded into the next-page keyset cursor.
type datePageRow struct {
	app PlanningApplication
	ac  string
}

func scanDatePageRow(row pgx.CollectableRow) (datePageRow, error) {
	var r datePageRow
	dest := append(appScanDest(&r.app), &r.ac)
	if err := row.Scan(dest...); err != nil {
		return datePageRow{}, err
	}
	return r, nil
}

// FindInZonePage returns one page of up to limit applications within radiusMetres
// of (latitude, longitude), ordered by the requested sort, plus an opaque
// sort-aware cursor for the next page (empty when exhausted). It generalises
// FindNearbyPage (which stays the legacy default-distance path with its own
// cursor) to the {distance, newest, oldest} sorts of epic #682 slice 1. Every
// sort keeps the ST_DWithin spatial predicate and a total order whose keyset
// predicate exactly matches its ORDER BY, so pages never overlap or gap. The
// cursor embeds the sort mode: replaying it under a different sort returns
// ErrCursorSortMismatch, and a malformed cursor returns ErrCursorInvalid, so the
// caller can return 400 rather than a mis-ordered page. It is authority-agnostic.
func (s *PostgresStore) FindInZonePage(ctx context.Context, latitude, longitude, radiusMetres float64, sort Sort, limit int, cursor string) ([]PlanningApplication, string, error) {
	if !sort.Supported() {
		return nil, "", fmt.Errorf("sort %q: %w", sort, ErrUnsupportedSort)
	}
	var c *pageCursor
	if cursor != "" {
		decoded, err := decodePageCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		if decoded.M != sort {
			return nil, "", fmt.Errorf("cursor sort %q, request sort %q: %w", decoded.M, sort, ErrCursorSortMismatch)
		}
		c = &decoded
	}
	if sort == SortDistance {
		return s.findDistanceZonePage(ctx, latitude, longitude, radiusMetres, limit, c)
	}
	if sort == SortStatus {
		return s.findStatusZonePage(ctx, latitude, longitude, radiusMetres, limit, c)
	}
	return s.findDateZonePage(ctx, latitude, longitude, radiusMetres, sort, limit, c)
}

// findDistanceZonePage serves SortDistance: the legacy KNN nearest-first query
// with a sort-aware (mode="distance") cursor on (distance, planit_name).
func (s *PostgresStore) findDistanceZonePage(ctx context.Context, latitude, longitude, radiusMetres float64, limit int, c *pageCursor) ([]PlanningApplication, string, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if c == nil {
		rows, err = s.db.Query(ctx, findNearbyFirstPageQuery, longitude, latitude, radiusMetres, limit)
	} else {
		dist, perr := strconv.ParseFloat(c.D, 64)
		if perr != nil {
			return nil, "", fmt.Errorf("parse distance cursor: %w: %w", ErrCursorInvalid, perr)
		}
		rows, err = s.db.Query(ctx, findNearbyKeysetQuery, longitude, latitude, radiusMetres, limit, dist, c.N)
	}
	if err != nil {
		return nil, "", fmt.Errorf("find applications by distance near (%v, %v): %w", latitude, longitude, err)
	}
	collected, err := pgx.CollectRows(rows, scanNearbyRow)
	if err != nil {
		return nil, "", fmt.Errorf("find applications by distance near (%v, %v): %w", latitude, longitude, err)
	}
	apps := make([]PlanningApplication, len(collected))
	for i := range collected {
		apps[i] = collected[i].app
	}
	next := ""
	if limit > 0 && len(collected) == limit {
		last := collected[len(collected)-1]
		next, err = encodePageCursor(pageCursor{
			M: SortDistance,
			D: strconv.FormatFloat(last.dist, 'g', -1, 64),
			N: last.app.Name,
		})
		if err != nil {
			return nil, "", fmt.Errorf("encode distance cursor: %w", err)
		}
	}
	return apps, next, nil
}

// findDateZonePage serves SortNewest/SortOldest: the start_date-ordered query with
// a keyset cursor on (start_date, authority_code, planit_name).
func (s *PostgresStore) findDateZonePage(ctx context.Context, latitude, longitude, radiusMetres float64, sort Sort, limit int, c *pageCursor) ([]PlanningApplication, string, error) {
	query, args := dateZoneQuery(sort, latitude, longitude, radiusMetres, limit, c)
	return s.collectKeysetPage(ctx, sort, latitude, longitude, query, args, limit, func(last datePageRow) pageCursor {
		return dateCursorOf(sort, last)
	})
}

// collectKeysetPage runs a date-projection keyset query (scanDatePageRow rows —
// appColumns + authority_code, shared by the start_date and app_state sorts) and
// returns the hydrated apps plus the next-page cursor, empty unless the page is
// full. cursorOf builds the continuation cursor from the last row of a full page;
// sort supplies error context only.
func (s *PostgresStore) collectKeysetPage(ctx context.Context, sort Sort, latitude, longitude float64, query string, args []any, limit int, cursorOf func(datePageRow) pageCursor) ([]PlanningApplication, string, error) {
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("find applications by %s near (%v, %v): %w", sort, latitude, longitude, err)
	}
	collected, err := pgx.CollectRows(rows, scanDatePageRow)
	if err != nil {
		return nil, "", fmt.Errorf("find applications by %s near (%v, %v): %w", sort, latitude, longitude, err)
	}
	apps := make([]PlanningApplication, len(collected))
	for i := range collected {
		apps[i] = collected[i].app
	}
	next := ""
	if limit > 0 && len(collected) == limit {
		next, err = encodePageCursor(cursorOf(collected[len(collected)-1]))
		if err != nil {
			return nil, "", fmt.Errorf("encode %s cursor: %w", sort, err)
		}
	}
	return apps, next, nil
}

// dateZoneQuery selects the first-page, non-null-keyset, or NULL-tail-keyset query
// for the sort and assembles its positional args. The base args ($1..$4) are
// longitude, latitude, radius, limit; the keyset args extend them.
func dateZoneQuery(sort Sort, latitude, longitude, radiusMetres float64, limit int, c *pageCursor) (string, []any) {
	base := []any{longitude, latitude, radiusMetres, limit}
	switch {
	case c == nil:
		if sort == SortNewest {
			return newestFirstQuery, base
		}
		return oldestFirstQuery, base
	case c.SD == nil:
		// Cursor sits in the NULL-start_date tail: keyset on the tiebreak only.
		args := append(base, c.AC, c.N)
		if sort == SortNewest {
			return newestKeysetNullQuery, args
		}
		return oldestKeysetNullQuery, args
	default:
		args := append(base, *c.SD, c.AC, c.N)
		if sort == SortNewest {
			return newestKeysetQuery, args
		}
		return oldestKeysetQuery, args
	}
}

// dateCursorOf builds the next-page keyset cursor from the last row of a full
// page: its start_date (nil for a NULL-start_date tail row), authority_code, and
// planit_name. start_date is formatted as an unambiguous "2006-01-02" so the next
// predicate compares against a ::date, free of timezone drift.
func dateCursorOf(sort Sort, last datePageRow) pageCursor {
	c := pageCursor{M: sort, AC: last.ac, N: last.app.Name}
	if last.app.StartDate != nil {
		sd := last.app.StartDate.Format("2006-01-02")
		c.SD = &sd
	}
	return c
}

// findStatusZonePage serves SortStatus: the app_state-then-start_date query with a
// mixed-direction keyset cursor on (app_state, start_date, authority_code,
// planit_name). It reuses the date-projection row scanner; app_state and start_date
// are already hydrated on the application.
func (s *PostgresStore) findStatusZonePage(ctx context.Context, latitude, longitude, radiusMetres float64, limit int, c *pageCursor) ([]PlanningApplication, string, error) {
	query, args := statusZoneQuery(latitude, longitude, radiusMetres, limit, c)
	return s.collectKeysetPage(ctx, SortStatus, latitude, longitude, query, args, limit, statusCursorOf)
}

// statusZoneQuery selects the first-page or the per-NULL-tail keyset query for
// SortStatus and assembles its positional args. The base args ($1..$4) are
// longitude, latitude, radius, limit; the keyset args extend them, their count and
// meaning matching the chosen query's NULL-tail case (see the status keyset query
// constants).
func statusZoneQuery(latitude, longitude, radiusMetres float64, limit int, c *pageCursor) (string, []any) {
	base := []any{longitude, latitude, radiusMetres, limit}
	switch {
	case c == nil:
		return statusFirstQuery, base
	case c.AS != nil && c.SD != nil:
		return statusKeysetQuery, append(base, *c.AS, *c.SD, c.AC, c.N)
	case c.AS != nil: // start_date NULL: cursor in its group's NULL-start_date tail.
		return statusKeysetDateNullQuery, append(base, *c.AS, c.AC, c.N)
	case c.SD != nil: // app_state NULL: cursor in the NULL-app_state tail.
		return statusKeysetStateNullQuery, append(base, *c.SD, c.AC, c.N)
	default: // both NULL: the deepest tail.
		return statusKeysetBothNullQuery, append(base, c.AC, c.N)
	}
}

// statusCursorOf builds the next-page keyset cursor from the last row of a full
// status page: its app_state (nil for a NULL-app_state tail row), start_date (nil
// for a NULL-start_date tail row), authority_code, and planit_name. start_date is
// formatted as an unambiguous "2006-01-02" so the next predicate compares against a
// ::date, free of timezone drift.
func statusCursorOf(last datePageRow) pageCursor {
	c := pageCursor{M: SortStatus, AC: last.ac, N: last.app.Name}
	if last.app.AppState != nil {
		as := *last.app.AppState
		c.AS = &as
	}
	if last.app.StartDate != nil {
		sd := last.app.StartDate.Format("2006-01-02")
		c.SD = &sd
	}
	return c
}
