package demoaccount

import (
	"math"
	"testing"
	"time"
)

func TestSeedApplications_AllInWestminsterAuthority(t *testing.T) {
	t.Parallel()
	apps := seedApplications(time.Now())
	if len(apps) != 5 {
		t.Fatalf("seed count: got %d, want 5", len(apps))
	}
	for _, a := range apps {
		if a.AreaID != seedAuthorityID || a.AreaName != seedAuthorityName {
			t.Errorf("%s: authority got id=%d name=%q", a.Name, a.AreaID, a.AreaName)
		}
		if a.Longitude == nil || a.Latitude == nil {
			t.Errorf("%s: missing coordinates", a.Name)
		}
	}
}

// TestSeedApplications_WithinDemoRadius guards the contract the production
// ST_DISTANCE query depends on: every seeded application must lie inside the
// demo zone, or FindNearby would return fewer than five rows in prod (where the
// fake here can't catch it). Distance is computed using the haversine formula.
func TestSeedApplications_WithinDemoRadius(t *testing.T) {
	t.Parallel()
	for _, a := range seedApplications(time.Now()) {
		d := haversineMetres(demoLatitude, demoLongitude, *a.Latitude, *a.Longitude)
		if d > demoRadiusMetres {
			t.Errorf("%s is %.0fm from demo centre, outside the %dm radius", a.Name, d, demoRadiusMetres)
		}
	}
}

func haversineMetres(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusMetres = 6_371_000
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := rad(lat2 - lat1)
	dLon := rad(lon2 - lon1)
	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusMetres * 2 * math.Asin(math.Sqrt(h))
}
