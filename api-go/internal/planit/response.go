package planit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

// planItResponse is the PlanIt list-endpoint envelope.
type planItResponse struct {
	Records []planItRecord `json:"records"`
	PageSiz *int           `json:"pg_sz"`
	From    *int           `json:"from"`
	To      *int           `json:"to"`
	Total   *int           `json:"total"`
}

// planItRecord is one PlanIt application record. Nullable upstream fields are
// pointers so an absent value maps to a nil domain pointer.
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

	// PlanIt's top-level "location" GeoJSON object is deliberately not decoded:
	// location_x/location_y already build the point.
	Reference    *string         `json:"reference"`
	Altid        json.RawMessage `json:"altid"`         // may be a JSON string OR array depending on scraper
	AssociatedID json.RawMessage `json:"associated_id"` // same string-or-array shape as Altid
	LastChanged  *string         `json:"last_changed"`  // same naive-UTC format as LastDifferent
	LastScraped  *string         `json:"last_scraped"`  // same naive-UTC format as LastDifferent
	ScraperName  *string         `json:"scraper_name"`
	OtherFields  map[string]any  `json:"other_fields"`
}

// restrictedOtherFieldsKeys are the other_fields keys PlanIt marks "See source"
// for Data-Protection reasons. Exactly these are stripped; every other key,
// including applicant_address and agent contact fields, is kept.
var restrictedOtherFieldsKeys = [...]string{"applicant_name", "agent_name", "case_officer"}

// stripRestrictedOtherFields returns a copy of fields with
// restrictedOtherFieldsKeys removed. An absent or empty (post-strip) map maps
// to nil, matching PlanningApplication.OtherFields' "absent means nil" contract.
func stripRestrictedOtherFields(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	out := maps.Clone(fields)
	for _, restricted := range restrictedOtherFieldsKeys {
		delete(out, restricted)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// rawJSONOrNil returns a copy of raw, or nil when raw is absent or the JSON
// literal null (PlanIt's shape for an unset altid).
func rawJSONOrNil(raw json.RawMessage) []byte {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil
	}
	return bytes.Clone(raw)
}

// parsePlanItInstantLenient maps an absent, empty or unparseable bookkeeping
// timestamp (LastChanged/LastScraped) to nil, unlike LastDifferent, so these
// fields never cause a record to be dropped.
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

// fieldError names the PlanIt field that failed to map.
type fieldError struct {
	Field string
	Err   error
}

func (e *fieldError) Error() string { return e.Field + ": " + e.Err.Error() }

func (e *fieldError) Unwrap() error { return e.Err }

// toDomain maps a PlanIt record to an applications.PlanningApplication
// (location_x is longitude, location_y latitude). An empty last_different maps
// to the zero time; a malformed one is an error.
func (r planItRecord) toDomain() (applications.PlanningApplication, error) {
	startDate, err := parseDateOnly(r.StartDate)
	if err != nil {
		return applications.PlanningApplication{}, &fieldError{Field: "start_date", Err: err}
	}
	decidedDate, err := parseDateOnly(r.DecidedDate)
	if err != nil {
		return applications.PlanningApplication{}, &fieldError{Field: "decided_date", Err: err}
	}
	consultedDate, err := parseDateOnly(r.ConsultedDate)
	if err != nil {
		return applications.PlanningApplication{}, &fieldError{Field: "consulted_date", Err: err}
	}
	var lastDifferent time.Time
	if r.LastDifferent != "" {
		lastDifferent, err = parsePlanItInstant(r.LastDifferent)
		if err != nil {
			return applications.PlanningApplication{}, &fieldError{Field: "last_different", Err: err}
		}
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
// TZ-bearing value (should PlanIt ever send one) still parses.
var planItInstantLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05.999999",
}

func parsePlanItInstant(value string) (time.Time, error) {
	for _, layout := range planItInstantLayouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised timestamp %q", value)
}

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
