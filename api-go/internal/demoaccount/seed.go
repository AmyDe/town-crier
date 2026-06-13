// Package demoaccount owns the App Store reviewer demo endpoint:
// GET /v1/demo-account (anonymous). On first call for the fixed demo identity it
// idempotently seeds Cosmos — a Pro-tier profile, a Westminster watch zone, and
// five fixed Westminster planning applications — then returns the zone plus the
// applications found within the zone via a spatial lookup. It mirrors the .NET
// TownCrier.Application.DemoAccount slice (GH#418 iteration 7).
//
// The seed is a deliberate write side effect of a GET: it provisions a stable,
// data-rich account so Apple's reviewer can exercise the paid experience without
// a subscription. Subsequent calls find the profile already present and skip the
// seed, so the endpoint is idempotent.
package demoaccount

import (
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

// Demo authority: every seeded application and the demo watch zone belong to
// Westminster City Council, so the spatial lookup is scoped to its partition.
const (
	seedAuthorityName = "Westminster City Council"
	seedAuthorityID   = 441
)

// strptr / dateptr return pointers to fixed seed values; the domain snapshot
// carries nullable fields as pointers.
func strptr(s string) *string { return &s }

func dateptr(year int, month time.Month, day int) *time.Time {
	d := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	return &d
}

func floatptr(f float64) *float64 { return &f }

// seedApplications builds the five fixed Westminster demo applications, mirroring
// .NET DemoSeedData.CreateApplications. lastDifferent is stamped with now (the
// value is not part of the response, but is persisted for fidelity). Coordinates
// place all five within the 2 km demo zone so the spatial lookup returns them.
func seedApplications(now time.Time) []applications.PlanningApplication {
	return []applications.PlanningApplication{
		{
			Name:          "24/05678/FULL",
			UID:           "demo-app-001",
			AreaName:      seedAuthorityName,
			AreaID:        seedAuthorityID,
			Address:       "10 Downing Street, London SW1A 2AA",
			Postcode:      strptr("SW1A 2AA"),
			Description:   "Replacement of entrance door and associated security improvements",
			AppType:       strptr("Full"),
			AppState:      strptr("Under consideration"),
			StartDate:     dateptr(2026, time.January, 15),
			ConsultedDate: dateptr(2026, time.February, 1),
			Longitude:     floatptr(-0.1276),
			Latitude:      floatptr(51.5034),
			LastDifferent: now,
		},
		{
			Name:          "24/05679/FULL",
			UID:           "demo-app-002",
			AreaName:      seedAuthorityName,
			AreaID:        seedAuthorityID,
			Address:       "Westminster Abbey, 20 Deans Yd, London SW1P 3PA",
			Postcode:      strptr("SW1P 3PA"),
			Description:   "Installation of new lighting system in the nave and restoration of stonework",
			AppType:       strptr("Listed Building"),
			AppState:      strptr("Permitted"),
			StartDate:     dateptr(2025, time.November, 1),
			DecidedDate:   dateptr(2026, time.February, 20),
			ConsultedDate: dateptr(2025, time.December, 1),
			Longitude:     floatptr(-0.1273),
			Latitude:      floatptr(51.4993),
			LastDifferent: now,
		},
		{
			Name:          "24/05680/FULL",
			UID:           "demo-app-003",
			AreaName:      seedAuthorityName,
			AreaID:        seedAuthorityID,
			Address:       "Buckingham Palace, London SW1A 1AA",
			Postcode:      strptr("SW1A 1AA"),
			Description:   "Erection of temporary scaffolding for facade cleaning and minor repairs to roof drainage",
			AppType:       strptr("Full"),
			AppState:      strptr("Undecided"),
			StartDate:     dateptr(2026, time.February, 10),
			Longitude:     floatptr(-0.1419),
			Latitude:      floatptr(51.5014),
			LastDifferent: now,
		},
		{
			Name:          "24/05681/FULL",
			UID:           "demo-app-004",
			AreaName:      seedAuthorityName,
			AreaID:        seedAuthorityID,
			Address:       "St James's Park, London SW1A 2BJ",
			Postcode:      strptr("SW1A 2BJ"),
			Description:   "Construction of new accessible footpath and installation of park benches",
			AppType:       strptr("Full"),
			AppState:      strptr("Rejected"),
			StartDate:     dateptr(2025, time.October, 5),
			DecidedDate:   dateptr(2026, time.January, 30),
			ConsultedDate: dateptr(2025, time.November, 1),
			Longitude:     floatptr(-0.1340),
			Latitude:      floatptr(51.5025),
			LastDifferent: now,
		},
		{
			Name:          "24/05682/FULL",
			UID:           "demo-app-005",
			AreaName:      seedAuthorityName,
			AreaID:        seedAuthorityID,
			Address:       "Trafalgar Square, London WC2N 5DN",
			Postcode:      strptr("WC2N 5DN"),
			Description:   "Replacement of paving stones around the central fountain with conditions on materials and working hours",
			AppType:       strptr("Full"),
			AppState:      strptr("Conditions"),
			StartDate:     dateptr(2025, time.September, 12),
			DecidedDate:   dateptr(2026, time.January, 18),
			ConsultedDate: dateptr(2025, time.October, 10),
			Longitude:     floatptr(-0.1281),
			Latitude:      floatptr(51.5080),
			LastDifferent: now,
		},
	}
}
