package planit

import (
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
