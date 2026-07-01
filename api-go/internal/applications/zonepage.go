package applications

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

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

// The status filter vocabulary (epic #682 slice 4). These are the PlanIt
// app_state chip values the clients filter on; ?status= exact-matches the raw
// app_state column against one of these. "All" (or an absent ?status=) means no
// status filter; any other value is rejected with 400 (see StatusSupported).
// app_state is a nullable raw string, so the match is on the exact stored value.
const (
	StatusUndecided  = "Undecided"
	StatusPermitted  = "Permitted"
	StatusConditions = "Conditions"
	StatusRejected   = "Rejected"
	StatusWithdrawn  = "Withdrawn"
	StatusAppealed   = "Appealed"
)

// StatusSupported reports whether status is one of the recognised app_state
// filter values. It is the concrete (non-"All", non-empty) vocabulary only:
// callers map "" / "All" to "no filter" before reaching this.
func StatusSupported(status string) bool {
	switch status {
	case StatusUndecided, StatusPermitted, StatusConditions, StatusRejected, StatusWithdrawn, StatusAppealed:
		return true
	default:
		return false
	}
}

// InZoneQuery is the full request descriptor for FindInZonePage: where to look
// (UserID, Latitude, Longitude, RadiusMetres), how to order (Sort), how to
// filter (Status, Unread), and how to page (Limit, Cursor). It replaces a long
// positional parameter list (epic #682 slice 4) so the call site reads as one
// query object rather than ten arguments.
//
// Status is the exact app_state to match; "" means no status filter. Unread true
// restricts to applications with an unread notification for UserID; false means
// no unread filter. Status and Unread are mutually exclusive at the API boundary
// (the handler rejects both together with 400) — the store does not assume so,
// but the cursor embeds both, so a cursor minted under one filter cannot be
// replayed under another (ErrCursorFilterMismatch).
type InZoneQuery struct {
	UserID       string
	Latitude     float64
	Longitude    float64
	RadiusMetres float64
	Sort         Sort
	Status       string
	Unread       bool
	Limit        int
	Cursor       string
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
	// ErrCursorFilterMismatch signals a cursor whose embedded filter (status
	// value and/or unread flag) differs from the request's filter. A keyset cursor
	// is only valid for the candidate set it was minted over; replaying it under a
	// different filter would gap or overlap pages, so consumers map this to HTTP
	// 400 — symmetric with ErrCursorSortMismatch, never a silent reset.
	ErrCursorFilterMismatch = errors.New("cursor filter does not match request filter")
	// ErrUnsupportedSort signals a sort value outside the set this slice
	// implements. Consumers map it to HTTP 400.
	ErrUnsupportedSort = errors.New("unsupported sort")
)

// pageCursor is the opaque, sort-aware keyset position handed back to clients.
// It embeds the sort mode (M) so a stale cursor cannot silently produce a page
// in the wrong order, the active filter (F = status value, U = unread flag) so a
// cursor minted over one candidate set cannot be replayed over another, plus
// exactly the keys that sort's ORDER BY ranks on:
//   - distance: D (the KNN distance) + N (planit_name).
//   - newest/oldest: SD (start_date, nil for a NULL-start_date tail row) + AC
//     (authority_code) + N (planit_name).
//   - status: AS (app_state, nil for a NULL-app_state tail row) + SD (start_date,
//     nil for a NULL-start_date tail row) + AC (authority_code) + N (planit_name).
//   - recent-activity: TS (the activity timestamp, nil for a NULL-activity tail
//     row) + AC (authority_code) + N (planit_name).
//
// It is base64url-encoded JSON. F and U are omitempty so an unfiltered cursor is
// byte-identical to the pre-slice-4 cursors (they decode to F="" U=false, which
// matches an unfiltered request — old cursors keep working). AS, SD and TS use a
// nil pointer (omitted field) to mean a NULL key value, so the next page's
// predicate can pick the matching NULLS-LAST tail branch. SD is a "2006-01-02"
// date string so the predicate compares against an unambiguous ::date, not a
// timezone-laden timestamp; TS is an RFC3339Nano timestamp string compared
// against a ::timestamptz.
type pageCursor struct {
	M  Sort    `json:"m"`
	F  string  `json:"f,omitempty"`
	U  bool    `json:"u,omitempty"`
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

// FindInZonePage returns one page of up to q.Limit applications within
// q.RadiusMetres of (q.Latitude, q.Longitude), ordered by q.Sort and filtered by
// q.Status / q.Unread, plus an opaque sort-and-filter-aware cursor for the next
// page (empty when exhausted). It generalises FindNearbyPage (which stays the
// legacy default-distance path with its own cursor) to the {distance, newest,
// oldest, status, recent-activity} sorts of epic #682 slices 1-3 and the status /
// unread filters of slice 4. Every sort keeps the ST_DWithin spatial predicate
// and a total order whose keyset predicate exactly matches its ORDER BY, so pages
// never overlap or gap; the filters compose by restricting the candidate set
// without touching the ordering. The cursor embeds the sort mode AND the active
// filter: replaying it under a different sort returns ErrCursorSortMismatch,
// under a different filter ErrCursorFilterMismatch, and a malformed cursor returns
// ErrCursorInvalid, so the caller can return 400 rather than a mis-ordered or
// gapped page. It is authority-agnostic.
//
// q.UserID scopes the per-user notification data the recent-activity sort joins
// (the caller's latest unread per application) and the unread filter restricts on.
// The scalar sorts ignore it unless q.Unread is set.
func (s *PostgresStore) FindInZonePage(ctx context.Context, q InZoneQuery) ([]PlanningApplication, string, error) {
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

	// Filtered requests (a status value and/or the unread flag) take the dynamic
	// builder, which composes the status predicate and/or the unread INNER JOIN
	// into the sort's query. Unfiltered requests stay on the proven per-sort
	// constant queries unchanged.
	if q.Status != "" || q.Unread {
		return s.findFilteredZonePage(ctx, q, c)
	}
	if q.Sort == SortDistance {
		return s.findDistanceZonePage(ctx, q.Latitude, q.Longitude, q.RadiusMetres, q.Limit, c)
	}
	if q.Sort == SortStatus {
		return s.findStatusZonePage(ctx, q.Latitude, q.Longitude, q.RadiusMetres, q.Limit, c)
	}
	if q.Sort == SortRecentActivity {
		return s.findRecentActivityZonePage(ctx, q.UserID, q.Latitude, q.Longitude, q.RadiusMetres, q.Limit, c)
	}
	return s.findDateZonePage(ctx, q.Latitude, q.Longitude, q.RadiusMetres, q.Sort, q.Limit, c)
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
	wrap := func(err error) error {
		return fmt.Errorf("find applications by distance near (%v, %v): %w", latitude, longitude, err)
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

// findDateZonePage serves SortNewest/SortOldest: the start_date-ordered query with
// a keyset cursor on (start_date, authority_code, planit_name).
func (s *PostgresStore) findDateZonePage(ctx context.Context, latitude, longitude, radiusMetres float64, sort Sort, limit int, c *pageCursor) ([]PlanningApplication, string, error) {
	query, args := dateZoneQuery(sort, latitude, longitude, radiusMetres, limit, c)
	return s.collectKeysetPage(ctx, sort, latitude, longitude, query, args, limit, func(last datePageRow) pageCursor {
		return dateCursorOf(sort, last)
	})
}

// nearbyAppOf, datePageAppOf and activityAppOf project the hydrated application
// out of each paged-row type for collectPage. Every row carries the application
// in its .app field; these adapters exist only because Go generics cannot read a
// struct field through a type parameter.
func nearbyAppOf(r nearbyRow) PlanningApplication     { return r.app }
func datePageAppOf(r datePageRow) PlanningApplication { return r.app }
func activityAppOf(r activityPageRow) PlanningApplication {
	return r.app
}

// collectPage is the shared tail of every paged zone read. It drains rows with
// scan, projects each row's application via appOf, and — only when the page came
// back full (limit > 0 && len == limit) — mints the next-page cursor from the last
// row via cursorOf. cursorOf owns its own encoding and error context (the cursor
// codec differs per call site: encodePageCursor for the sort-aware cursors,
// encodeNearbyCursor for the legacy FindNearbyPage), so its ("", err) propagates
// unchanged. A row-scan failure is wrapped with wrap, so each call site keeps its
// existing "find ... near (lat, lon)" error context byte-for-byte.
func collectPage[T any](
	rows pgx.Rows,
	scan func(pgx.CollectableRow) (T, error),
	appOf func(T) PlanningApplication,
	cursorOf func(T) (string, error),
	limit int,
	wrap func(error) error,
) ([]PlanningApplication, string, error) {
	collected, err := pgx.CollectRows(rows, scan)
	if err != nil {
		return nil, "", wrap(err)
	}
	apps := make([]PlanningApplication, len(collected))
	for i := range collected {
		apps[i] = appOf(collected[i])
	}
	next := ""
	if limit > 0 && len(collected) == limit {
		next, err = cursorOf(collected[len(collected)-1])
		if err != nil {
			return nil, "", err
		}
	}
	return apps, next, nil
}

// collectKeysetPage runs a date-projection keyset query (scanDatePageRow rows —
// appColumns + authority_code, shared by the start_date and app_state sorts) and
// returns the hydrated apps plus the next-page cursor, empty unless the page is
// full. cursorOf builds the continuation cursor from the last row of a full page;
// sort supplies error context only. It routes the collect-and-mint tail through
// collectPage, adapting cursorOf (a pageCursor builder) into the encode-and-wrap
// closure collectPage expects.
func (s *PostgresStore) collectKeysetPage(ctx context.Context, sort Sort, latitude, longitude float64, query string, args []any, limit int, cursorOf func(datePageRow) pageCursor) ([]PlanningApplication, string, error) {
	wrap := func(err error) error {
		return fmt.Errorf("find applications by %s near (%v, %v): %w", sort, latitude, longitude, err)
	}
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, "", wrap(err)
	}
	return collectPage(rows, scanDatePageRow, datePageAppOf,
		func(last datePageRow) (string, error) {
			enc, err := encodePageCursor(cursorOf(last))
			if err != nil {
				return "", fmt.Errorf("encode %s cursor: %w", sort, err)
			}
			return enc, nil
		},
		limit, wrap)
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

// activityUnreadSubquery computes, per application, the caller's latest UNREAD
// notification timestamp. $5 is the caller's userID. Unread is read_at IS NULL
// (ADR 0035, replacing the notification_state watermark JOIN). The "no unread
// for an untouched user" behaviour is preserved by migration 0015's backfill:
// existing no-watermark users had their history marked read (read_at set), so
// they contribute no rows here; a genuinely new notification (read_at IS NULL)
// correctly reads as unread. It groups on (application_uid, authority_id) — the
// same keys the outer LEFT JOIN matches against the application's (uid, area_id)
// — so the integer authority_id (NOT the text authority_code) reconciles a
// notification to its application, and a bare uid shared across authorities
// cannot cross-contaminate.
const activityUnreadSubquery = "SELECT n.application_uid, n.authority_id, MAX(n.created_at) AS created_at" +
	" FROM notifications n" +
	" WHERE n.user_id = $5 AND n.read_at IS NULL" +
	" GROUP BY n.application_uid, n.authority_id"

// activityExpr is the recent-activity sort key: the later of the application's
// start_date (cast to timestamptz) and the caller's latest unread for it. Postgres
// GREATEST ignores NULL inputs, so an app with no unread orders by start_date
// alone, an app with a newer unread floats up, and an app with neither is NULL
// (sorted last under NULLS LAST). u.created_at is the subquery's per-app unread
// timestamp; start_date resolves to applications (no name collision with u).
const activityExpr = "GREATEST(start_date::timestamptz, u.created_at)"

// activityFromWhere is the shared SELECT + LEFT JOIN + spatial predicate for the
// recent-activity sort. The projection is dateColumns (appColumns + authority_code,
// matching scanActivityPageRow's app+ac scan) plus the computed activity_ts. $1 is
// longitude and $2 latitude (via nearbyPoint), $3 the radius, $4 the limit, $5 the
// userID (in the join subquery). The LEFT JOIN keeps apps with no unread (their
// activity is start_date alone, or NULL).
const activityFromWhere = "SELECT " + dateColumns + ", " + activityExpr + " AS activity_ts" +
	" FROM applications" +
	" LEFT JOIN (" + activityUnreadSubquery + ") u" +
	" ON applications.uid = u.application_uid AND applications.area_id = u.authority_id" +
	" WHERE ST_DWithin(location, " + nearbyPoint + ", $3)"

// activityOrderBy is the ORDER BY + LIMIT suffix for SortRecentActivity. NULLS LAST
// keeps NULL-activity rows after every active row; (authority_code, planit_name) is
// the unique tiebreak.
const activityOrderBy = " ORDER BY " + activityExpr + " DESC NULLS LAST, authority_code, planit_name LIMIT $4"

// recentActivityFirstQuery is the recent-activity first page: join + spatial
// predicate, ordered, limited.
const recentActivityFirstQuery = activityFromWhere + activityOrderBy

// Keyset queries resuming after a cursor. The predicate exactly mirrors the DESC
// NULLS LAST order so pages never overlap or gap: rows further along the activity
// order, OR all NULL-activity rows (they trail in NULLS LAST), OR an equal-activity
// row past the (authority_code, planit_name) tiebreak. The GREATEST expression is
// repeated inline because Postgres cannot reference the SELECT alias in WHERE.
const (
	// $6 the cursor's activity_ts (::timestamptz), $7 authority_code, $8 planit_name.
	recentActivityKeysetQuery = activityFromWhere +
		" AND (" + activityExpr + " < $6::timestamptz OR " + activityExpr + " IS NULL" +
		" OR (" + activityExpr + " = $6::timestamptz AND (authority_code, planit_name) > ($7, $8)))" +
		activityOrderBy
	// $6 authority_code, $7 planit_name. Cursor in the NULL-activity tail, where the
	// only remaining order is the (authority_code, planit_name) tiebreak.
	recentActivityKeysetNullQuery = activityFromWhere +
		" AND " + activityExpr + " IS NULL AND (authority_code, planit_name) > ($6, $7)" +
		activityOrderBy
)

// activityPageRow carries a hydrated application plus its authority_code and the
// computed recent-activity timestamp (NULL when the app has neither a start_date
// nor an unread), so the last row of a full page can be encoded into the next-page
// keyset cursor.
type activityPageRow struct {
	app      PlanningApplication
	ac       string
	activity *time.Time
}

func scanActivityPageRow(row pgx.CollectableRow) (activityPageRow, error) {
	var r activityPageRow
	dest := append(appScanDest(&r.app), &r.ac, &r.activity)
	if err := row.Scan(dest...); err != nil {
		return activityPageRow{}, err
	}
	return r, nil
}

// findRecentActivityZonePage serves SortRecentActivity: the GREATEST(start_date,
// unread.created_at) DESC NULLS LAST query with a keyset cursor on (activity_ts,
// authority_code, planit_name). userID scopes the joined unread notifications.
func (s *PostgresStore) findRecentActivityZonePage(ctx context.Context, userID string, latitude, longitude, radiusMetres float64, limit int, c *pageCursor) ([]PlanningApplication, string, error) {
	query, args := recentActivityZoneQuery(userID, latitude, longitude, radiusMetres, limit, c)
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

// recentActivityZoneQuery selects the first-page, non-null-keyset, or
// NULL-tail-keyset query and assembles its positional args. The base args ($1..$5)
// are longitude, latitude, radius, limit, userID; the keyset args extend them.
func recentActivityZoneQuery(userID string, latitude, longitude, radiusMetres float64, limit int, c *pageCursor) (string, []any) {
	base := []any{longitude, latitude, radiusMetres, limit, userID}
	switch {
	case c == nil:
		return recentActivityFirstQuery, base
	case c.TS == nil:
		// Cursor sits in the NULL-activity tail: keyset on the tiebreak only.
		return recentActivityKeysetNullQuery, append(base, c.AC, c.N)
	default:
		return recentActivityKeysetQuery, append(base, *c.TS, c.AC, c.N)
	}
}

// activityCursorOf builds the next-page keyset cursor from the last row of a full
// recent-activity page: its activity timestamp (nil for a NULL-activity tail row),
// authority_code, and planit_name. The timestamp is formatted as RFC3339Nano in
// UTC so the next predicate compares against an unambiguous ::timestamptz at the
// same instant the database produced.
func activityCursorOf(last activityPageRow) pageCursor {
	c := pageCursor{M: SortRecentActivity, AC: last.ac, N: last.app.Name}
	if last.activity != nil {
		ts := last.activity.UTC().Format(time.RFC3339Nano)
		c.TS = &ts
	}
	return c
}

// argBuilder accumulates positional query args and hands out their $N
// placeholders, so the filtered FindInZonePage path can assemble a query that
// interleaves spatial, join, status-filter and keyset parameters without
// hand-numbering them. Only constant SQL fragments and these $N placeholders are
// concatenated into the query text; every value flows through the args, so the
// query stays fully parameterised.
type argBuilder struct {
	args []any
}

func (b *argBuilder) add(v any) string {
	b.args = append(b.args, v)
	return "$" + strconv.Itoa(len(b.args))
}

// findFilteredZonePage serves the status / unread filtered requests (epic #682
// slice 4). It composes the same per-sort ORDER BY + keyset predicates the
// unfiltered path uses with a status predicate (AND app_state = $x) and/or an
// INNER JOIN to the caller's unread notifications, then collects with the sort's
// projection scanner and mints a filter-stamped next cursor. c is the validated
// (sort- and filter-matched) cursor, nil on the first page.
func (s *PostgresStore) findFilteredZonePage(ctx context.Context, q InZoneQuery, c *pageCursor) ([]PlanningApplication, string, error) {
	query, args, err := buildFilteredZoneQuery(q, c)
	if err != nil {
		return nil, "", err
	}
	switch q.Sort {
	case SortDistance:
		return s.collectFilteredDistancePage(ctx, q, query, args)
	case SortRecentActivity:
		return s.collectFilteredActivityPage(ctx, q, query, args)
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

// buildFilteredZoneQuery assembles the SQL + positional args for a status/unread
// filtered page. The shape mirrors the unfiltered constants exactly — same
// projection, ORDER BY and keyset predicates — but with two composable additions:
// an optional "AND app_state = $x" status predicate, and an optional unread join.
// The join is INNER when q.Unread is set (it drops applications with no unread for
// the caller) and LEFT when only recent-activity needs it for ordering; both reuse
// the read_at IS NULL unread subquery (ADR 0035), grouped on (application_uid,
// authority_id) so the outer join stays authority-safe.
func buildFilteredZoneQuery(q InZoneQuery, c *pageCursor) (string, []any, error) {
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

	radiusP := b.add(q.RadiusMetres)
	sb.WriteString(" WHERE ST_DWithin(location, " + point + ", " + radiusP + ")")

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

// orderByExpr returns the ORDER BY column list (without LIMIT) for the sort. It
// is the exact ordering the unfiltered constants use, so a filtered page is
// ordered identically to its unfiltered counterpart — the filter only restricts
// the candidate set.
func orderByExpr(sort Sort, point string) string {
	switch sort {
	case SortDistance:
		return "location <-> " + point + ", planit_name"
	case SortNewest:
		return "start_date DESC NULLS LAST, authority_code, planit_name"
	case SortOldest:
		return "start_date ASC NULLS LAST, authority_code, planit_name"
	case SortStatus:
		return "app_state ASC NULLS LAST, start_date DESC NULLS LAST, authority_code, planit_name"
	case SortRecentActivity:
		return activityExpr + " DESC NULLS LAST, authority_code, planit_name"
	default:
		return ""
	}
}

// keysetPredicate builds the "strictly after the cursor" predicate for the sort,
// assigning its parameters via b. It mirrors the unfiltered keyset constants
// exactly (each column's direction and NULLS-LAST tail), so a filtered page never
// overlaps or gaps. A distance cursor's stored decimal is parsed here; a malformed
// one is ErrCursorInvalid.
func keysetPredicate(sort Sort, point string, c *pageCursor, b *argBuilder) (string, error) {
	switch sort {
	case SortDistance:
		dist, err := strconv.ParseFloat(c.D, 64)
		if err != nil {
			return "", fmt.Errorf("parse distance cursor: %w: %w", ErrCursorInvalid, err)
		}
		distP := b.add(dist)
		nameP := b.add(c.N)
		expr := "location <-> " + point
		return "(" + expr + " > " + distP +
			" OR (" + expr + " = " + distP + " AND planit_name > " + nameP + "))", nil
	case SortNewest, SortOldest:
		return dateKeysetPredicate(sort, c, b), nil
	case SortStatus:
		return statusKeysetPredicate(c, b), nil
	case SortRecentActivity:
		return activityKeysetPredicate(c, b), nil
	default:
		return "", fmt.Errorf("sort %q: %w", sort, ErrUnsupportedSort)
	}
}

// dateKeysetPredicate mirrors newestKeysetQuery / oldestKeysetQuery (and their
// NULL-start_date tail forms) with dynamic parameters.
func dateKeysetPredicate(sort Sort, c *pageCursor, b *argBuilder) string {
	if c.SD == nil {
		acP := b.add(c.AC)
		nameP := b.add(c.N)
		return "start_date IS NULL AND (authority_code, planit_name) > (" + acP + ", " + nameP + ")"
	}
	cmp := "<"
	if sort == SortOldest {
		cmp = ">"
	}
	sdP := b.add(*c.SD)
	acP := b.add(c.AC)
	nameP := b.add(c.N)
	return "(start_date " + cmp + " " + sdP + "::date OR start_date IS NULL" +
		" OR (start_date = " + sdP + "::date AND (authority_code, planit_name) > (" + acP + ", " + nameP + ")))"
}

// statusKeysetPredicate mirrors the four status keyset constants (the cursor row's
// app_state/start_date NULLS-LAST position) with dynamic parameters.
func statusKeysetPredicate(c *pageCursor, b *argBuilder) string {
	switch {
	case c.AS != nil && c.SD != nil:
		asP := b.add(*c.AS)
		sdP := b.add(*c.SD)
		acP := b.add(c.AC)
		nameP := b.add(c.N)
		return "(app_state > " + asP + " OR app_state IS NULL" +
			" OR (app_state = " + asP + " AND (start_date < " + sdP + "::date OR start_date IS NULL))" +
			" OR (app_state = " + asP + " AND start_date = " + sdP + "::date AND (authority_code, planit_name) > (" + acP + ", " + nameP + ")))"
	case c.AS != nil: // start_date NULL: cursor in its group's NULL-start_date tail.
		asP := b.add(*c.AS)
		acP := b.add(c.AC)
		nameP := b.add(c.N)
		return "(app_state > " + asP + " OR app_state IS NULL" +
			" OR (app_state = " + asP + " AND start_date IS NULL AND (authority_code, planit_name) > (" + acP + ", " + nameP + ")))"
	case c.SD != nil: // app_state NULL: cursor in the NULL-app_state tail.
		sdP := b.add(*c.SD)
		acP := b.add(c.AC)
		nameP := b.add(c.N)
		return "app_state IS NULL AND ((start_date < " + sdP + "::date OR start_date IS NULL)" +
			" OR (start_date = " + sdP + "::date AND (authority_code, planit_name) > (" + acP + ", " + nameP + ")))"
	default: // both NULL: the deepest tail.
		acP := b.add(c.AC)
		nameP := b.add(c.N)
		return "app_state IS NULL AND start_date IS NULL AND (authority_code, planit_name) > (" + acP + ", " + nameP + ")"
	}
}

// activityKeysetPredicate mirrors recentActivityKeysetQuery / its NULL-activity
// tail form with dynamic parameters. The GREATEST expression is repeated inline
// because Postgres cannot reference the SELECT alias in WHERE.
func activityKeysetPredicate(c *pageCursor, b *argBuilder) string {
	if c.TS == nil {
		acP := b.add(c.AC)
		nameP := b.add(c.N)
		return activityExpr + " IS NULL AND (authority_code, planit_name) > (" + acP + ", " + nameP + ")"
	}
	tsP := b.add(*c.TS)
	acP := b.add(c.AC)
	nameP := b.add(c.N)
	return "(" + activityExpr + " < " + tsP + "::timestamptz OR " + activityExpr + " IS NULL" +
		" OR (" + activityExpr + " = " + tsP + "::timestamptz AND (authority_code, planit_name) > (" + acP + ", " + nameP + ")))"
}

// collectFilteredDistancePage runs a filtered distance query (nearbyRow
// projection) and mints a filter-stamped (distance, planit_name) next cursor.
func (s *PostgresStore) collectFilteredDistancePage(ctx context.Context, q InZoneQuery, query string, args []any) ([]PlanningApplication, string, error) {
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

// collectFilteredActivityPage runs a filtered recent-activity query (activity
// projection) and mints a filter-stamped (activity_ts, authority_code,
// planit_name) next cursor.
func (s *PostgresStore) collectFilteredActivityPage(ctx context.Context, q InZoneQuery, query string, args []any) ([]PlanningApplication, string, error) {
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
