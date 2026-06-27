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
// Slice 1 (epic #682) implements {distance, newest, oldest}. The remaining UI
// sorts (status, recent-activity) arrive in later slices; until then they are
// rejected as unsupported.
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
)

// Supported reports whether s is a sort this slice implements server-side.
func (s Sort) Supported() bool {
	switch s {
	case SortDistance, SortNewest, SortOldest:
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
//
// It is base64url-encoded JSON. SD is a "2006-01-02" date string so the next
// page's predicate compares against an unambiguous ::date, not a timezone-laden
// timestamp.
type pageCursor struct {
	M  Sort    `json:"m"`
	D  string  `json:"d,omitempty"`
	SD *string `json:"sd,omitempty"`
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
		next, err = encodePageCursor(dateCursorOf(sort, collected[len(collected)-1]))
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
