package demoaccount

import "github.com/AmyDe/town-crier/api-go/internal/applications"

// demoAccountResult is the wire shape of GET /v1/demo-account. JSON keys are
// camelCase; tier is the SubscriptionTier string ("Pro").
type demoAccountResult struct {
	UserID       string                  `json:"userId"`
	Tier         string                  `json:"tier"`
	WatchZone    demoWatchZoneResult     `json:"watchZone"`
	Applications []demoApplicationResult `json:"applications"`
}

// demoWatchZoneResult is the watch-zone projection within the demo response.
// authorityName is the council display name (not the stored zone name).
type demoWatchZoneResult struct {
	ZoneID        string  `json:"zoneId"`
	AuthorityName string  `json:"authorityName"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	RadiusMetres  float64 `json:"radiusMetres"`
}

// demoApplicationResult is the trimmed application projection the demo endpoint
// returns (no coordinates, dates, or unread-event data). appType and appState
// are nullable, marshalling to null when absent.
type demoApplicationResult struct {
	UID         string  `json:"uid"`
	Name        string  `json:"name"`
	Address     string  `json:"address"`
	Description string  `json:"description"`
	AppType     *string `json:"appType"`
	AppState    *string `json:"appState"`
}

// applicationResultOf maps a domain snapshot to its demo wire projection
// (uid, name, address, description, appType, appState).
func applicationResultOf(a applications.PlanningApplication) demoApplicationResult {
	return demoApplicationResult{
		UID:         a.UID,
		Name:        a.Name,
		Address:     a.Address,
		Description: a.Description,
		AppType:     a.AppType,
		AppState:    a.AppState,
	}
}
