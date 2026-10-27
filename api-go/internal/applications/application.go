// Package applications owns the planning-application snapshot, its Postgres
// store, and the application read endpoints.
//
// A PlanningApplication is a snapshot of a PlanIt case: a plain data carrier
// (the values come from an external provider, validated at the HTTP boundary),
// not a rich aggregate.
package applications

import (
	"bytes"
	"encoding/json"
	"strconv"
	"time"
)

// PlanningApplication is the planning-application snapshot. Nullable fields are
// pointers so an absent value serialises as JSON null.
type PlanningApplication struct {
	Name          string
	UID           string
	AreaName      string
	AreaID        int
	Address       string
	Postcode      *string
	Description   string
	AppType       *string
	AppState      *string
	AppSize       *string
	StartDate     *time.Time
	DecidedDate   *time.Time
	ConsultedDate *time.Time
	Longitude     *float64
	Latitude      *float64
	URL           *string
	Link          *string
	LastDifferent time.Time

	// The fields below are every remaining top-level PlanIt core field plus
	// other_fields, minus the three DP-restricted keys. All are silent (see
	// HasSameSilentFieldsAs): a change in only these updates the stored row
	// without notification fan-out (ADR 0021, the reindex-flood guard).
	Reference    *string    // PlanIt's own "reference" field (distinct from Name/UID)
	Altid        []byte     // raw JSON: PlanIt's altid may be a string OR an array depending on scraper
	AssociatedID []byte     // raw JSON: same string-or-array shape as Altid
	LastChanged  *time.Time // PlanIt bookkeeping timestamp; excluded from both comparison methods
	LastScraped  *time.Time // PlanIt bookkeeping timestamp; excluded from both comparison methods
	ScraperName  *string
	OtherFields  map[string]any // PlanIt's other_fields map, verbatim minus applicant_name/agent_name/case_officer
}

// CanonicalUID is the server-derived identity "{AreaID}/{Name}". PlanIt case
// references are only unique within a council, so the authority must be part of
// the key. This is deliberately independent of the raw UID field — a client may
// send a stale-format uid, but two saves of the same (AreaID, Name) always
// produce the same canonical uid, keeping the saved-application doc id stable
// and re-saves idempotent.
func (a PlanningApplication) CanonicalUID() string {
	return strconv.Itoa(a.AreaID) + "/" + a.Name
}

// HasSameBusinessFieldsAs reports whether every business-material field matches
// other, ignoring LastDifferent. The poll cycle uses it to skip a redundant
// upsert when PlanIt re-emits an application with only a bumped LastDifferent
// timestamp — the load-bearing reindex-flood guard.
func (a PlanningApplication) HasSameBusinessFieldsAs(other PlanningApplication) bool {
	return a.Name == other.Name &&
		a.UID == other.UID &&
		a.AreaName == other.AreaName &&
		a.AreaID == other.AreaID &&
		a.Address == other.Address &&
		eqPtr(a.Postcode, other.Postcode) &&
		a.Description == other.Description &&
		eqPtr(a.AppType, other.AppType) &&
		eqPtr(a.AppState, other.AppState) &&
		eqPtr(a.AppSize, other.AppSize) &&
		eqTimePtr(a.StartDate, other.StartDate) &&
		eqTimePtr(a.DecidedDate, other.DecidedDate) &&
		eqTimePtr(a.ConsultedDate, other.ConsultedDate) &&
		eqPtr(a.Longitude, other.Longitude) &&
		eqPtr(a.Latitude, other.Latitude) &&
		eqPtr(a.URL, other.URL) &&
		eqPtr(a.Link, other.Link)
}

// HasSameSilentFieldsAs reports whether every silent field matches other:
// Reference, Altid, AssociatedID, ScraperName, and OtherFields. LastChanged and LastScraped are deliberately
// EXCLUDED — they are bookkeeping, alongside LastDifferent, and bump on
// effectively every PlanIt re-emission; including them here would make every
// PlanIt bulk re-index a row write, defeating the reindex-flood guard the
// ingester relies on. The poll cycle uses this method, alongside
// HasSameBusinessFieldsAs, to classify a change as silent (upsert, no
// notification fan-out) vs bookkeeping-only (no write at all).
func (a PlanningApplication) HasSameSilentFieldsAs(other PlanningApplication) bool {
	return eqPtr(a.Reference, other.Reference) &&
		eqRawJSON(a.Altid, other.Altid) &&
		eqRawJSON(a.AssociatedID, other.AssociatedID) &&
		eqPtr(a.ScraperName, other.ScraperName) &&
		eqOtherFields(a.OtherFields, other.OtherFields)
}

// eqRawJSON reports whether two raw-JSON byte slices represent the same JSON
// value, treating nil and empty as equal. It must not be bytes.Equal: PlanIt's
// pretty-printed JSON is kept verbatim while Postgres jsonb reformats it, so a
// byte comparison would find array-valued altid/associated_id unequal on every
// re-fetch and defeat the silent-change write suppression. Both sides are
// normalised by unmarshal and re-marshal; unparseable JSON is treated as
// not-equal.
func eqRawJSON(a, b []byte) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	var aVal, bVal any
	if err := json.Unmarshal(a, &aVal); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bVal); err != nil {
		return false
	}
	aBytes, aErr := json.Marshal(aVal)
	bBytes, bErr := json.Marshal(bVal)
	if aErr != nil || bErr != nil {
		return false
	}
	return bytes.Equal(aBytes, bBytes)
}

// eqOtherFields reports whether two other_fields maps are equal, treating nil
// and empty as equal. json.Marshal sorts map keys, so insertion order is
// irrelevant; a marshal failure is treated as not-equal, which costs at most one
// extra silent-field upsert rather than masking a real change.
func eqOtherFields(a, b map[string]any) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	aBytes, aErr := json.Marshal(a)
	bBytes, bErr := json.Marshal(b)
	if aErr != nil || bErr != nil {
		return false
	}
	return bytes.Equal(aBytes, bBytes)
}

// eqPtr reports whether two optional values are equal (both nil, or both set
// to the same value).
func eqPtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// eqTimePtr reports whether two optional date values are equal.
func eqTimePtr(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}
