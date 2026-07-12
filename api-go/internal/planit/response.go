package planit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

// planItResponse is the PlanIt list-endpoint envelope. JSON tags mirror the
// snake_case keys PlanIt returns.
type planItResponse struct {
	Records []planItRecord `json:"records"`
	PageSiz *int           `json:"pg_sz"`
	From    *int           `json:"from"`
	Total   *int           `json:"total"`
}

// planItRecord is one PlanIt application record. JSON tags match PlanIt's
// snake_case field names. Nullable upstream fields are pointers so an absent
// value maps to a nil domain pointer.
type planItRecord struct {
	Name          string   `json:"name"`
	UID           string   `json:"uid"`
	AreaName      string   `json:"area_name"`
	AreaID        int      `json:"area_id"`
	Address       string   `json:"address"`
	Postcode      *string  `json:"postcode"`
	Description   string   `json:"description"`
	AppType       string   `json:"app_type"`
	AppState      string   `json:"app_state"`
	AppSize       *string  `json:"app_size"`
	StartDate     *string  `json:"start_date"`
	DecidedDate   *string  `json:"decided_date"`
	ConsultedDate *string  `json:"consulted_date"`
	LocationX     *float64 `json:"location_x"`
	LocationY     *float64 `json:"location_y"`
	URL           *string  `json:"url"`
	Link          *string  `json:"link"`
	LastDifferent string   `json:"last_different"`

	// The fields below are the PlanIt full-field widening (GH#935). All are
	// carried into applications.PlanningApplication's silent fields verbatim
	// (minus the three DP-restricted OtherFields keys); the top-level
	// "location" GeoJSON object PlanIt also returns is deliberately absent
	// here (no json tag) so it is ignored — location_x/location_y already
	// build the geography point, so it is redundant.
	Reference    *string         `json:"reference"`
	Altid        json.RawMessage `json:"altid"`         // may be a JSON string OR array depending on scraper
	AssociatedID json.RawMessage `json:"associated_id"` // same string-or-array shape as Altid
	LastChanged  *string         `json:"last_changed"`  // same naive-UTC format as LastDifferent; bookkeeping
	LastScraped  *string         `json:"last_scraped"`  // same naive-UTC format as LastDifferent; bookkeeping
	ScraperName  *string         `json:"scraper_name"`
	OtherFields  map[string]any  `json:"other_fields"`
}

// restrictedOtherFieldsKeys are the three PlanIt other_fields keys carrying
// Data-Protection-restricted values (PlanIt itself returns the literal
// placeholder "See source" for each). Owner decision 2026-07-12: strip
// exactly these three keys — nothing else — verbatim-minus-three has zero
// ambiguity, and every other other_fields key (including applicant_address,
// agent company/address/tel fields, and coordinate duplicates) is kept.
var restrictedOtherFieldsKeys = [...]string{"applicant_name", "agent_name", "case_officer"}

// stripRestrictedOtherFields returns a copy of fields with
// restrictedOtherFieldsKeys removed. An absent or empty (post-strip) map maps
// to nil, matching PlanningApplication.OtherFields' "absent means nil" contract.
func stripRestrictedOtherFields(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]any, len(fields))
	for k, v := range fields {
		out[k] = v
	}
	for _, restricted := range restrictedOtherFieldsKeys {
		delete(out, restricted)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// rawJSONOrNil returns raw's bytes, or nil when raw is absent or its value is
// the JSON literal null — so an explicit "altid": null (PlanIt's real shape
// when the field is unset) maps to a nil domain pointer rather than the
// 4-byte literal "null". json.RawMessage always captures a JSON value's exact
// matched token with no surrounding whitespace, so a plain byte comparison is
// sufficient here.
func rawJSONOrNil(raw json.RawMessage) []byte {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil
	}
	return append([]byte(nil), raw...)
}

// parsePlanItInstantLenient parses an optional PlanIt bookkeeping timestamp
// (LastChanged/LastScraped), tolerating an absent, empty, or unparseable
// value by mapping it to nil rather than failing the whole record — unlike
// LastDifferent, these fields are pure PlanIt bookkeeping (GH#935) and must
// never cause a real application to be dropped.
func parsePlanItInstantLenient(value *string) *time.Time {
	if value == nil || *value == "" {
		return nil
	}
	t, err := parsePlanItInstant(*value)
	if err != nil {
		return nil
	}
	return &t
}

// toDomain maps a PlanIt record to the applications.PlanningApplication snapshot:
// location_x is longitude, location_y latitude; app_type / app_state are carried
// as non-empty pointers; date-only fields parse to *time.Time; last_different
// parses as a UTC instant, tolerating the no-timezone fractional-second form
// PlanIt actually emits.
func (r planItRecord) toDomain() (applications.PlanningApplication, error) {
	startDate, err := parseDateOnly(r.StartDate)
	if err != nil {
		return applications.PlanningApplication{}, fmt.Errorf("start_date: %w", err)
	}
	decidedDate, err := parseDateOnly(r.DecidedDate)
	if err != nil {
		return applications.PlanningApplication{}, fmt.Errorf("decided_date: %w", err)
	}
	consultedDate, err := parseDateOnly(r.ConsultedDate)
	if err != nil {
		return applications.PlanningApplication{}, fmt.Errorf("consulted_date: %w", err)
	}
	lastDifferent, err := parsePlanItInstant(r.LastDifferent)
	if err != nil {
		return applications.PlanningApplication{}, fmt.Errorf("last_different %q: %w", r.LastDifferent, err)
	}

	appType := nonEmptyPtr(r.AppType)
	appState := nonEmptyPtr(r.AppState)

	return applications.PlanningApplication{
		Name:          r.Name,
		UID:           r.UID,
		AreaName:      r.AreaName,
		AreaID:        r.AreaID,
		Address:       r.Address,
		Postcode:      r.Postcode,
		Description:   r.Description,
		AppType:       appType,
		AppState:      appState,
		AppSize:       r.AppSize,
		StartDate:     startDate,
		DecidedDate:   decidedDate,
		ConsultedDate: consultedDate,
		Longitude:     r.LocationX,
		Latitude:      r.LocationY,
		URL:           r.URL,
		Link:          r.Link,
		LastDifferent: lastDifferent,

		Reference:    r.Reference,
		Altid:        rawJSONOrNil(r.Altid),
		AssociatedID: rawJSONOrNil(r.AssociatedID),
		LastChanged:  parsePlanItInstantLenient(r.LastChanged),
		LastScraped:  parsePlanItInstantLenient(r.LastScraped),
		ScraperName:  r.ScraperName,
		OtherFields:  stripRestrictedOtherFields(r.OtherFields),
	}, nil
}

// planItInstantLayouts are tried in order when parsing PlanIt's last_different.
// PlanIt emits a naive UTC timestamp with variable fractional seconds and NO
// timezone (e.g. "2026-06-13T00:06:34.112581"); RFC3339Nano is kept first so a
// TZ-bearing value (should PlanIt ever send one) still parses. tc-8l96.
var planItInstantLayouts = []string{
	time.RFC3339Nano,             // TZ present, optional fractional seconds
	"2006-01-02T15:04:05.999999", // no TZ, optional fractional seconds -> UTC
}

// parsePlanItInstant parses a PlanIt last_different timestamp as UTC, tolerating
// the no-timezone fractional-second form PlanIt actually returns.
func parsePlanItInstant(value string) (time.Time, error) {
	for _, layout := range planItInstantLayouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised timestamp %q", value)
}

// parseDateOnly parses an optional "yyyy-MM-dd" PlanIt date into a *time.Time,
// returning nil for an absent or empty value.
func parseDateOnly(value *string) (*time.Time, error) {
	if value == nil || *value == "" {
		return nil, nil //nolint:nilnil // absent optional date is a valid nil, not an error
	}
	t, err := time.Parse("2006-01-02", *value)
	if err != nil {
		return nil, fmt.Errorf("parse date %q: %w", *value, err)
	}
	return &t, nil
}

// nonEmptyPtr returns a pointer to s, or nil when s is empty, so an empty
// upstream string maps to a JSON-null domain field.
func nonEmptyPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
