package authorities

// Lookup is the exported, cross-package handle for resolving authority ids to
// their display metadata. Features such as application-authorities depend on it
// without reaching into the package's unexported static store.
type Lookup struct {
	store *staticStore
}

// NewLookup builds an authority lookup over the embedded static authority data.
func NewLookup() *Lookup {
	return &Lookup{store: newStaticStore()}
}

// ByID returns the authority with the given id, reporting whether it exists.
func (l *Lookup) ByID(id int) (Authority, bool) {
	return l.store.byID(id)
}

// All returns every authority in the static list, sorted by name. The polling
// all-authority provider filters this set down to the pollable area types.
func (l *Lookup) All() []Authority {
	return l.store.all()
}

// SlugToAreaID resolves an authority slug (as produced by Slugify over the
// authority Name) to its area id — the authority ID — reporting whether the slug
// is known. On a slug collision the first authority by list order wins (see
// newStaticStore). Returns (0, false) for an unknown slug.
func (l *Lookup) SlugToAreaID(slug string) (int, bool) {
	return l.store.slugToAreaID(slug)
}

// SlugForAreaID returns Slugify(Name) for the authority with the given id,
// reporting whether the id is known. Returns ("", false) for an unknown id.
func (l *Lookup) SlugForAreaID(id int) (string, bool) {
	return l.store.slugForAreaID(id)
}

// CompareOrdinalIgnoreCase exposes the package's ordinal, case-insensitive name
// comparator so callers building authority lists order them identically to the
// /v1/authorities list ordering.
func CompareOrdinalIgnoreCase(a, b string) int {
	return compareOrdinalIgnoreCase(a, b)
}
