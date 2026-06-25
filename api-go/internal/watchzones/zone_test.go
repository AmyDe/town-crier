package watchzones

import (
	"testing"
	"time"
)

func testZone(t *testing.T) WatchZone {
	t.Helper()
	z, err := NewWatchZone(
		"zone-1", "auth0|user", "Home",
		51.5074, -0.1278, 1000, 471,
		time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC),
		true, true)
	if err != nil {
		t.Fatalf("NewWatchZone: %v", err)
	}
	return z
}

func TestWatchZone_BoundingBox(t *testing.T) {
	t.Parallel()
	// A 1 km zone centred on central London. The bounding box is the centre offset
	// by the radius converted to degrees: latitude is a constant scale, longitude
	// is scaled by cos(latitude), so at 51.5° the east-west span is wider than the
	// north-south span. Expected values precomputed from the documented formula:
	//   dLat = 1000 / 111320                      = 0.0089831
	//   dLon = 1000 / (111320 * cos(51.5074°))    = 0.0144328
	z, err := NewWatchZone("z1", "u1", "Home", 51.5074, -0.1278, 1000, 471,
		time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC), true, true)
	if err != nil {
		t.Fatalf("NewWatchZone: %v", err)
	}
	minLat, maxLat, minLon, maxLon := z.boundingBox()

	const tol = 1e-4
	for _, c := range []struct {
		name string
		got  float64
		want float64
	}{
		{"minLat", minLat, 51.4984169},
		{"maxLat", maxLat, 51.5163831},
		{"minLon", minLon, -0.1422328},
		{"maxLon", maxLon, -0.1133672},
	} {
		if diff := c.got - c.want; diff > tol || diff < -tol {
			t.Errorf("%s: got %v, want %v (±%v)", c.name, c.got, c.want, tol)
		}
	}
	// The box must be symmetric about the centre and the longitude span wider than
	// the latitude span at UK latitudes (cos(lat) < 1).
	if maxLat-z.Latitude <= 0 || (maxLat-z.Latitude)-(z.Latitude-minLat) > tol {
		t.Errorf("latitude box not symmetric about centre: [%v,%v] centre %v", minLat, maxLat, z.Latitude)
	}
	if (maxLon - minLon) <= (maxLat - minLat) {
		t.Errorf("longitude span must exceed latitude span at UK latitudes: lon=%v lat=%v", maxLon-minLon, maxLat-minLat)
	}
}

func TestNewWatchZone_Validation(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name        string
		id          string
		userID      string
		zoneName    string
		radius      float64
		authorityID int
		wantErr     bool
	}{
		{"valid", "z1", "u1", "Home", 500, 471, false},
		{"blank id", "", "u1", "Home", 500, 471, true},
		{"whitespace id", "  ", "u1", "Home", 500, 471, true},
		{"blank user", "z1", "", "Home", 500, 471, true},
		{"blank name", "z1", "u1", "", 500, 471, true},
		{"whitespace name", "z1", "u1", "   ", 500, 471, true},
		{"zero radius", "z1", "u1", "Home", 0, 471, true},
		{"negative radius", "z1", "u1", "Home", -5, 471, true},
		{"zero authority", "z1", "u1", "Home", 500, 0, true},
		{"negative authority", "z1", "u1", "Home", 500, -3, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewWatchZone(tc.id, tc.userID, tc.zoneName, 51, -0.1, tc.radius, tc.authorityID, now, true, true)
			if (err != nil) != tc.wantErr {
				t.Fatalf("got err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestNewWatchZone_DoesNotValidateCoordinateRange(t *testing.T) {
	t.Parallel()
	// Latitude/longitude range is an HTTP-layer concern, not a domain invariant.
	// The constructor accepts any coordinate.
	now := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	if _, err := NewWatchZone("z1", "u1", "Home", 999, -999, 500, 471, now, true, true); err != nil {
		t.Fatalf("constructor must not range-check coordinates: %v", err)
	}
}

func TestWatchZone_WithUpdates_PartialMergePreservesUnsetFields(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	newName := "Office"
	newRadius := 2500.0
	push := false

	updated, err := z.WithUpdates(ZoneUpdate{
		Name:         &newName,
		RadiusMetres: &newRadius,
		PushEnabled:  &push,
	})
	if err != nil {
		t.Fatalf("WithUpdates: %v", err)
	}

	if updated.Name != "Office" {
		t.Errorf("name: got %q, want Office", updated.Name)
	}
	if updated.RadiusMetres != 2500 {
		t.Errorf("radius: got %v, want 2500", updated.RadiusMetres)
	}
	if updated.PushEnabled != false {
		t.Errorf("pushEnabled: got %v, want false", updated.PushEnabled)
	}
	// Unset fields preserved.
	if updated.Latitude != z.Latitude || updated.Longitude != z.Longitude {
		t.Errorf("coords changed: got (%v,%v), want (%v,%v)", updated.Latitude, updated.Longitude, z.Latitude, z.Longitude)
	}
	if updated.AuthorityID != z.AuthorityID {
		t.Errorf("authorityId changed: got %d, want %d", updated.AuthorityID, z.AuthorityID)
	}
	if updated.EmailInstantEnabled != z.EmailInstantEnabled {
		t.Errorf("emailInstantEnabled changed: got %v, want %v", updated.EmailInstantEnabled, z.EmailInstantEnabled)
	}
	// Identity and creation timestamp are immutable across an update.
	if updated.ID != z.ID || updated.UserID != z.UserID || !updated.CreatedAt.Equal(z.CreatedAt) {
		t.Errorf("identity/createdAt mutated: %+v", updated)
	}
}

func TestWatchZone_WithUpdates_RejectsInvalidMergeResult(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	blank := "   "
	if _, err := z.WithUpdates(ZoneUpdate{Name: &blank}); err == nil {
		t.Fatal("WithUpdates must re-validate: blank name should error")
	}
}

func TestWatchZone_WithUpdates_EmptyUpdateIsIdentity(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	updated, err := z.WithUpdates(ZoneUpdate{})
	if err != nil {
		t.Fatalf("WithUpdates: %v", err)
	}
	if updated != z {
		t.Errorf("empty update changed zone: got %+v, want %+v", updated, z)
	}
}
