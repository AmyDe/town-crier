// Package authorities serves GET /v1/authorities and GET /v1/authorities/{id}
// from an embedded authorities.json. The list is sorted by name with ordinal,
// case-insensitive ordering so the wire order is stable.
package authorities

import (
	"embed"
	"encoding/json"
	"fmt"
	"slices"
)

//go:embed resources/authorities.json
var resources embed.FS

// Authority is the embedded record. councilUrl and planningUrl are absent from
// authorities.json so they are not stored here; the detail handler emits
// explicit nulls for wire compatibility.
type Authority struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	AreaType string `json:"areaType"`
}

// staticStore holds the authorities pre-sorted by name (ordinal-ignore-case)
// and indexed by id for O(1) point lookups.
type staticStore struct {
	sorted []Authority
	byIDx  map[int]Authority
}

// newStaticStore loads and indexes the embedded authorities once. It panics on
// a malformed embedded resource — a build-time invariant, the only sanctioned
// panic site.
func newStaticStore() *staticStore {
	raw, err := resources.ReadFile("resources/authorities.json")
	if err != nil {
		panic(fmt.Sprintf("authorities: read embedded resource: %v", err))
	}
	var records []Authority
	if err := json.Unmarshal(raw, &records); err != nil {
		panic(fmt.Sprintf("authorities: unmarshal embedded resource: %v", err))
	}

	slices.SortStableFunc(records, func(a, b Authority) int {
		return compareOrdinalIgnoreCase(a.Name, b.Name)
	})

	byIDx := make(map[int]Authority, len(records))
	for _, a := range records {
		byIDx[a.ID] = a
	}

	return &staticStore{sorted: records, byIDx: byIDx}
}

func (s *staticStore) all() []Authority {
	return s.sorted
}

func (s *staticStore) byID(id int) (Authority, bool) {
	a, ok := s.byIDx[id]
	return a, ok
}

// compareOrdinalIgnoreCase sorts ASCII strings by code unit after uppercasing,
// falling back to length. Returns -1, 0, or 1.
func compareOrdinalIgnoreCase(a, b string) int {
	n := min(len(a), len(b))
	for i := range n {
		ca, cb := asciiUpper(a[i]), asciiUpper(b[i])
		if ca != cb {
			if ca < cb {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	default:
		return 0
	}
}

func asciiUpper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - ('a' - 'A')
	}
	return c
}
