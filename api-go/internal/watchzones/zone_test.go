package watchzones

import (
	"errors"
	"reflect"
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
	// WatchZone now carries a slice field (Boundary), so it is no longer
	// comparable with == / !=; reflect.DeepEqual is the structural equivalent.
	if !reflect.DeepEqual(updated, z) {
		t.Errorf("empty update changed zone: got %+v, want %+v", updated, z)
	}
}

// --- Boundary (custom-shape) tests ---------------------------------------

// londonSquare is a small closed square near central London, well within UK
// bounds, symmetric about (51.5074, -0.1278) with an offset chosen so it is
// roughly a 1km-radius shape (matching the boundingBox test's precomputed
// formula: dLat = r/111320, dLon = r/(111320*cos(lat))).
func londonSquare(t *testing.T) []Coordinate {
	t.Helper()
	const (
		centreLat = 51.5074
		centreLon = -0.1278
		dLat      = 0.0089831
		dLon      = 0.0144328
	)
	return []Coordinate{
		{Longitude: centreLon + dLon, Latitude: centreLat + dLat},
		{Longitude: centreLon - dLon, Latitude: centreLat + dLat},
		{Longitude: centreLon - dLon, Latitude: centreLat - dLat},
		{Longitude: centreLon + dLon, Latitude: centreLat - dLat},
	}
}

func TestNewBoundary_RejectsFewerThanThreeVertices(t *testing.T) {
	t.Parallel()
	_, err := NewBoundary([]Coordinate{
		{Longitude: -0.1, Latitude: 51.5},
		{Longitude: -0.11, Latitude: 51.51},
	})
	if err == nil {
		t.Fatal("NewBoundary must reject a 2-vertex ring")
	}
	if !errors.Is(err, ErrBoundaryVertexCount) {
		t.Errorf("got err=%v, want ErrBoundaryVertexCount", err)
	}
}

func TestNewBoundary_RejectsMoreThanFiftyVertices(t *testing.T) {
	t.Parallel()
	vertices := make([]Coordinate, 0, 51)
	for i := range 51 {
		vertices = append(vertices, Coordinate{
			Longitude: -0.5 + float64(i)*0.001,
			Latitude:  51.5,
		})
	}
	_, err := NewBoundary(vertices)
	if err == nil {
		t.Fatal("NewBoundary must reject a 51-vertex ring")
	}
	if !errors.Is(err, ErrBoundaryVertexCount) {
		t.Errorf("got err=%v, want ErrBoundaryVertexCount", err)
	}
}

func TestNewBoundary_RejectsSelfIntersectingRing(t *testing.T) {
	t.Parallel()
	// A bowtie: the two diagonals of this quadrilateral cross in the middle.
	_, err := NewBoundary([]Coordinate{
		{Longitude: -0.10, Latitude: 51.50},
		{Longitude: -0.09, Latitude: 51.51},
		{Longitude: -0.09, Latitude: 51.50},
		{Longitude: -0.10, Latitude: 51.51},
	})
	if err == nil {
		t.Fatal("NewBoundary must reject a self-intersecting ring")
	}
	if !errors.Is(err, ErrBoundarySelfIntersecting) {
		t.Errorf("got err=%v, want ErrBoundarySelfIntersecting", err)
	}
}

func TestNewBoundary_RejectsOutOfUKBoundsVertex(t *testing.T) {
	t.Parallel()
	_, err := NewBoundary([]Coordinate{
		{Longitude: -0.1, Latitude: 51.5},
		{Longitude: -0.11, Latitude: 51.51},
		{Longitude: 40.0, Latitude: 51.5}, // well east of the UK
	})
	if err == nil {
		t.Fatal("NewBoundary must reject a vertex outside UK bounds")
	}
	if !errors.Is(err, ErrBoundaryOutOfBounds) {
		t.Errorf("got err=%v, want ErrBoundaryOutOfBounds", err)
	}
}

func TestNewBoundary_RejectsDuplicateVertex(t *testing.T) {
	t.Parallel()
	_, err := NewBoundary([]Coordinate{
		{Longitude: -0.10, Latitude: 51.50},
		{Longitude: -0.09, Latitude: 51.51},
		{Longitude: -0.09, Latitude: 51.51}, // duplicate of the previous vertex
		{Longitude: -0.08, Latitude: 51.50},
	})
	if err == nil {
		t.Fatal("NewBoundary must reject a duplicate interior vertex")
	}
	if !errors.Is(err, ErrBoundaryDuplicateVertex) {
		t.Errorf("got err=%v, want ErrBoundaryDuplicateVertex", err)
	}
}

func TestNewBoundary_AutoClosesUnclosedRing(t *testing.T) {
	t.Parallel()
	triangle := []Coordinate{
		{Longitude: -0.10, Latitude: 51.50},
		{Longitude: -0.09, Latitude: 51.51},
		{Longitude: -0.09, Latitude: 51.50},
	}
	b, err := NewBoundary(triangle)
	if err != nil {
		t.Fatalf("NewBoundary: %v", err)
	}
	if len(b) != len(triangle)+1 {
		t.Fatalf("got %d vertices, want %d (auto-closed)", len(b), len(triangle)+1)
	}
	if b[0] != b[len(b)-1] {
		t.Errorf("ring not closed: first=%v last=%v", b[0], b[len(b)-1])
	}
}

func TestNewBoundary_AlreadyClosedRingIsNotDoubled(t *testing.T) {
	t.Parallel()
	closed := []Coordinate{
		{Longitude: -0.10, Latitude: 51.50},
		{Longitude: -0.09, Latitude: 51.51},
		{Longitude: -0.09, Latitude: 51.50},
		{Longitude: -0.10, Latitude: 51.50}, // already closes the ring
	}
	b, err := NewBoundary(closed)
	if err != nil {
		t.Fatalf("NewBoundary: %v", err)
	}
	if len(b) != len(closed) {
		t.Fatalf("got %d vertices, want %d (should not double the closing vertex)", len(b), len(closed))
	}
}

func TestBoundary_Centroid_KnownSquare(t *testing.T) {
	t.Parallel()
	square := Boundary{
		{Longitude: 0, Latitude: 0},
		{Longitude: 0, Latitude: 10},
		{Longitude: 10, Latitude: 10},
		{Longitude: 10, Latitude: 0},
		{Longitude: 0, Latitude: 0},
	}
	lat, lon := square.Centroid()
	const want = 5.0
	if diff := lat - want; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("lat: got %v, want %v", lat, want)
	}
	if diff := lon - want; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("lon: got %v, want %v", lon, want)
	}
}

func TestBoundary_Centroid_AsymmetricPolygonNotVertexAverage(t *testing.T) {
	t.Parallel()
	// A staircase/L-shape: (0,0),(6,0),(6,2),(2,2),(2,4),(0,4) in (lon,lat).
	// The true polygon centroid is (2.5, 1.5); the naive vertex average is
	// (2.667, 2.0) — different enough that a naive-average implementation
	// fails this test.
	shape := Boundary{
		{Longitude: 0, Latitude: 0},
		{Longitude: 6, Latitude: 0},
		{Longitude: 6, Latitude: 2},
		{Longitude: 2, Latitude: 2},
		{Longitude: 2, Latitude: 4},
		{Longitude: 0, Latitude: 4},
		{Longitude: 0, Latitude: 0},
	}
	lat, lon := shape.Centroid()
	const (
		wantLat = 1.5
		wantLon = 2.5
		tol     = 1e-9
	)
	if diff := lat - wantLat; diff > tol || diff < -tol {
		t.Errorf("lat: got %v, want %v", lat, wantLat)
	}
	if diff := lon - wantLon; diff > tol || diff < -tol {
		t.Errorf("lon: got %v, want %v", lon, wantLon)
	}
}

func TestBoundary_EnclosingRadiusMetres_KnownSquare(t *testing.T) {
	t.Parallel()
	square, err := NewBoundary(londonSquare(t))
	if err != nil {
		t.Fatalf("NewBoundary: %v", err)
	}
	got := square.EnclosingRadiusMetres()
	const (
		want = 1412.694247415168
		tol  = 0.5
	)
	if diff := got - want; diff > tol || diff < -tol {
		t.Errorf("got %v, want %v (±%v)", got, want, tol)
	}
}

func TestWatchZone_IsCustomShape(t *testing.T) {
	t.Parallel()
	circle := testZone(t)
	if circle.IsCustomShape() {
		t.Error("a circle zone (nil Boundary) must not report IsCustomShape")
	}

	shape, err := circle.WithBoundary(londonSquare(t))
	if err != nil {
		t.Fatalf("WithBoundary: %v", err)
	}
	if !shape.IsCustomShape() {
		t.Error("a zone with a non-nil Boundary must report IsCustomShape")
	}
}

func TestWatchZone_WithBoundary_DerivesCentroidAndRadius(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	shape, err := z.WithBoundary(londonSquare(t))
	if err != nil {
		t.Fatalf("WithBoundary: %v", err)
	}
	const (
		wantLat    = 51.5074
		wantLon    = -0.1278
		wantRadius = 1412.694247415168
		tol        = 0.5
	)
	if diff := shape.Latitude - wantLat; diff > 1e-4 || diff < -1e-4 {
		t.Errorf("latitude: got %v, want %v", shape.Latitude, wantLat)
	}
	if diff := shape.Longitude - wantLon; diff > 1e-4 || diff < -1e-4 {
		t.Errorf("longitude: got %v, want %v", shape.Longitude, wantLon)
	}
	if diff := shape.RadiusMetres - wantRadius; diff > tol || diff < -tol {
		t.Errorf("radiusMetres: got %v, want %v", shape.RadiusMetres, wantRadius)
	}
	// Identity and other fields carry over unchanged.
	if shape.ID != z.ID || shape.UserID != z.UserID || shape.Name != z.Name {
		t.Errorf("identity fields changed: got %+v", shape)
	}
}

func TestWatchZone_WithBoundary_InvalidVerticesReturnsError(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	if _, err := z.WithBoundary([]Coordinate{{Longitude: -0.1, Latitude: 51.5}}); err == nil {
		t.Fatal("WithBoundary must reject an invalid ring")
	}
}

func TestWatchZone_WithUpdates_BoundaryTriState(t *testing.T) {
	t.Parallel()
	circleZone := testZone(t)
	shapeZone, err := circleZone.WithBoundary(londonSquare(t))
	if err != nil {
		t.Fatalf("WithBoundary: %v", err)
	}

	t.Run("absent leaves shape untouched", func(t *testing.T) {
		t.Parallel()
		updated, err := shapeZone.WithUpdates(ZoneUpdate{})
		if err != nil {
			t.Fatalf("WithUpdates: %v", err)
		}
		if !reflect.DeepEqual(updated.Boundary, shapeZone.Boundary) {
			t.Errorf("boundary changed by an absent update: got %v, want %v", updated.Boundary, shapeZone.Boundary)
		}
	})

	t.Run("explicit null clears the boundary and keeps the derived centroid/radius", func(t *testing.T) {
		t.Parallel()
		cleared := Boundary{}
		updated, err := shapeZone.WithUpdates(ZoneUpdate{Boundary: &cleared})
		if err != nil {
			t.Fatalf("WithUpdates: %v", err)
		}
		if updated.IsCustomShape() {
			t.Errorf("boundary not cleared: %v", updated.Boundary)
		}
		if updated.Latitude != shapeZone.Latitude || updated.Longitude != shapeZone.Longitude {
			t.Errorf("centre changed on clear: got (%v,%v), want (%v,%v)",
				updated.Latitude, updated.Longitude, shapeZone.Latitude, shapeZone.Longitude)
		}
		if updated.RadiusMetres != shapeZone.RadiusMetres {
			t.Errorf("radius changed on clear: got %v, want %v", updated.RadiusMetres, shapeZone.RadiusMetres)
		}
	})

	t.Run("a value sets a new boundary and recomputes centroid/radius", func(t *testing.T) {
		t.Parallel()
		newShape := Boundary(londonSquare(t))
		updated, err := circleZone.WithUpdates(ZoneUpdate{Boundary: &newShape})
		if err != nil {
			t.Fatalf("WithUpdates: %v", err)
		}
		if !updated.IsCustomShape() {
			t.Fatal("boundary not set")
		}
		if updated.Latitude == circleZone.Latitude && updated.Longitude == circleZone.Longitude {
			t.Error("centre was not recomputed from the new boundary")
		}
	})

	t.Run("an invalid value is rejected", func(t *testing.T) {
		t.Parallel()
		tooFew := Boundary{{Longitude: -0.1, Latitude: 51.5}}
		if _, err := circleZone.WithUpdates(ZoneUpdate{Boundary: &tooFew}); err == nil {
			t.Fatal("WithUpdates must reject an invalid boundary value")
		}
	})
}
