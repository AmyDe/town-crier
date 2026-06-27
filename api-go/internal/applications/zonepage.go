package applications

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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
