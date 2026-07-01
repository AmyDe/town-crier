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

// staticStore holds the authorities pre-sorted by name (ordinal-ignore-case),
// indexed by id for O(1) point lookups, and indexed both ways between slug and
// id. The slug maps are built once at construction, never per call.
type staticStore struct {
	sorted   []Authority
	byIDx    map[int]Authority
	slugToID map[string]int
	idToSlug map[int]string
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
	slugToID := make(map[string]int, len(records))
	idToSlug := make(map[int]string, len(records))
	for _, a := range records {
		byIDx[a.ID] = a

		slug := Slugify(a.Name)
		idToSlug[a.ID] = slug
		// Collision policy: if two authority names slugify to the same slug, the
		// FIRST by store order wins. records is already sorted by name
		// (ordinal-ignore-case), so the winner is deterministic. Collisions are
		// not expected among LPA names, but this keeps resolution stable if one
		// ever appears.
		if _, exists := slugToID[slug]; !exists {
			slugToID[slug] = a.ID
		}
	}

	return &staticStore{
		sorted:   records,
		byIDx:    byIDx,
		slugToID: slugToID,
		idToSlug: idToSlug,
	}
}

func (s *staticStore) all() []Authority {
	return s.sorted
}

func (s *staticStore) byID(id int) (Authority, bool) {
	a, ok := s.byIDx[id]
	return a, ok
}

func (s *staticStore) slugToAreaID(slug string) (int, bool) {
	id, ok := s.slugToID[slug]
	return id, ok
}

func (s *staticStore) slugForAreaID(id int) (string, bool) {
	slug, ok := s.idToSlug[id]
	return slug, ok
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
