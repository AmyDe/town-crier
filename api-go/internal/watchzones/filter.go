package watchzones

// FilterKey identifies one of the seven pre-canned, Pro-tier watch-zone
// filters (GH#1090, epic tc-w825j). The zero value ("") means "unfiltered" —
// it deliberately is not a member of filterCatalog; callers gate on a
// non-empty value before consulting the catalog (see IsValidFilterKey).
type FilterKey string

// The seven catalog filter keys. See filterCatalog for each one's display
// name, description, and to_tsquery expression.
const (
	FilterKeyFewerNotifications    FilterKey = "fewer_notifications"
	FilterKeyHouseBuilder          FilterKey = "house_builder"
	FilterKeyNewHomes              FilterKey = "new_homes"
	FilterKeyExtensionsAlterations FilterKey = "extensions_alterations"
	FilterKeyLoftExtension         FilterKey = "loft_extension"
	FilterKeyKitchenExtension      FilterKey = "kitchen_extension"
	FilterKeyHMOHouseShares        FilterKey = "hmo_house_shares"
)

// FilterDefinition is one catalog entry: the copy shown to the user plus the
// Postgres full-text expression that decides whether an application matches.
type FilterDefinition struct {
	// DisplayName is the short, user-facing label for the filter.
	DisplayName string
	// Description explains what the filter surfaces, in plain English.
	Description string
	// Expression is a to_tsquery('english', ...) boolean expression matched
	// against an application's description via
	// SELECT to_tsvector('english', $1) @@ to_tsquery('english', $2) — see
	// applications.PostgresStore.MatchesExpression (later bead). Empty for
	// FilterKeyFewerNotifications, whose only criterion is the app_type gate
	// below; every other entry's expression is non-empty.
	Expression string
}

// filterExcludedAppTypes lists the applications.PlanningApplication.AppType
// values that never generate a notification for any keyword-bearing filter
// (every catalog entry except FilterKeyFewerNotifications, which *is* this
// gate). Checked as a plain Go string-slice membership test against the
// already-loaded application — never as a SQL NOT-expression, which would
// force a sequential scan and bypass the applications_description_fts GIN
// index (see filter.go package doc / GH#1090 Technical Context).
var filterExcludedAppTypes = []string{"Trees", "Conditions", "Amendment", "Advertising", "Telecoms"}

// filterCatalog is the seven pre-canned watch-zone filters, validated
// against 1.4M rows of real town_crier_prod data (GH#1090). It is
// deliberately Go code, not a DB table: the catalog is meant to be revisable
// (adding/removing/retuning a filter) without a migration — see migration
// 0026_watch_zone_filter_key.sql, which adds an unconstrained filter_key
// column for exactly this reason.
//
// to_tsquery syntax gotchas — read before editing an expression below:
//
//   - `<N>` is EXACT token distance, not "within N". `discharge <2>
//     condition` matches "Discharge OF Condition No. 15" but not "Discharge
//     Conditions 18" (no "of"). Only `hip <2> gable` (loft_extension) relies
//     on a `<N>` phrase for anything load-bearing, and it was verified
//     against both "Hip to gable" and "Hip-to-gable" spellings in prod.
//   - Hyphenation splits into three tokens: to_tsvector('english',
//     'non-material') -> 'non-materi' 'non' 'materi'. None of the
//     expressions below need a hyphenated compound today, but a future one
//     should use the unhyphenated adjacency form (`non <-> material`) to
//     catch both spellings, not a literal hyphen.
//   - `&` is document-wide AND, not proximity — two terms anywhere in the
//     same description both count, even 200+ characters apart. This is why
//     the app_type gate (filterExcludedAppTypes) matters even for filters
//     that look precise on paper.
//   - A bare lexeme subsumes any phrase containing it — e.g. `(loft <->
//     conversion) | loft` would be redundant. No expression below carries
//     both a bare word and a phrase built from it.
//   - English stemming collapses plurals and some unrelated forms onto the
//     same lexeme (e.g. `flat`/`flats`), which is why new_homes deliberately
//     excludes bare `flat` — see GH#1090 for the false-positive analysis.
//
// Do not "clean up" or paraphrase these operators — every expression here
// was validated against live production data and a change silently
// invalidates that validation. Retuning the catalog is a separate,
// deliberate follow-up bead, not a drive-by edit.
var filterCatalog = map[FilterKey]FilterDefinition{
	FilterKeyFewerNotifications: {
		DisplayName: "Fewer notifications",
		Description: "Cuts out tree works, conditions, amendments, advertising and telecoms applications — general noise reduction with no keyword matching, just the app_type gate.",
		Expression:  "",
	},
	FilterKeyHouseBuilder: {
		DisplayName: "House builder",
		Description: "General contractor — deliberately broad: extensions, conversions, refurbishments, new builds, dwellings and more.",
		Expression:  "extension | conversion | refurbishment | (new <-> build) | dwelling | outbuilding | garage | basement | dormer | conservatory | annexe | erection | demolition",
	},
	FilterKeyNewHomes: {
		DisplayName: "New homes",
		Description: "Housing-supply / new-build watcher — erection, construction, redevelopment or development of dwellings, plus new-build, residential development and self-build mentions.",
		Expression:  "((erection | construction | redevelopment | development | demolition) & (dwelling | dwellinghouse | house | apartment | bungalow)) | (new <-> build) | (residential <-> development) | (self <-> build)",
	},
	FilterKeyExtensionsAlterations: {
		DisplayName: "Extensions & alterations",
		Description: "Neighbour tracking nearby building work — extensions, loft and garage conversions, outbuildings, conservatories, dormers, porches, annexes, alterations, garden rooms, verandas and pergolas.",
		Expression:  "extension | (loft <-> conversion) | (garage <-> conversion) | outbuilding | conservatory | dormer | porch | annexe | alteration | (garden <-> room) | veranda | pergola",
	},
	FilterKeyLoftExtension: {
		DisplayName: "Loft extension",
		Description: "Loft conversion specialist — loft, dormer, hip-to-gable, roofspace and roof space mentions.",
		Expression:  "loft | dormer | (hip <2> gable) | roofspace | (roof <-> space)",
	},
	FilterKeyKitchenExtension: {
		DisplayName: "Kitchen extension",
		Description: "We can't detect kitchen work specifically from a planning description. This shows every rear or single-storey extension nearby — most involve a kitchen.",
		Expression:  "extension & (rear | (single <-> storey))",
	},
	FilterKeyHMOHouseShares: {
		DisplayName: "HMO / house shares",
		Description: "HMO-sensitive streets — houses in multiple occupation, Class C4, and large Sui Generis bedroom/occupation/lodging applications.",
		Expression:  "hmo | (multiple <-> occupation) | (class <-> c4) | ((sui <-> generis) & (bedroom | occupation | lodging))",
	},
}

// IsValidFilterKey reports whether key is a member of the catalog. The
// empty string is never valid: "" means "no filter", and callers must gate
// on a non-empty value before calling IsValidFilterKey (per GH#1090's
// create/patch gating logic: `if req.FilterKey != nil && *req.FilterKey !=
// "" { ... IsValidFilterKey ... }`).
func IsValidFilterKey(key string) bool {
	_, ok := filterCatalog[FilterKey(key)]
	return ok
}
