// Package watchzones owns the watch-zone feature: the domain model, the Postgres
// store over the watch_zones table, and the /v1/me/watch-zones HTTP handlers
// (GH#418 iteration 5). It follows idiomatic Go — a plain struct validated at
// construction, a consumer-side store interface, and hand-written test fakes.
//
// Scope note: POST create (whose response body carries nearby applications) and
// GET /{zoneId}/applications are deferred to bead tc-5847 — they hard-depend on
// the geo/application stores that land in later iterations. This package ships
// list, update (PATCH), and delete; per-zone notification preferences live on
// the user profile and are served by the profiles package.
package watchzones

import (
	"errors"
	"math"
	"strings"
	"time"
)

// ErrNotFound signals that no watch zone exists for the given (user, zone) pair.
var ErrNotFound = errors.New("watch zone not found")

// Boundary validation errors. All are returned by NewBoundary (and therefore
// by WatchZone.WithBoundary and WithUpdates when a new boundary value is
// supplied); consumers should use errors.Is rather than string matching.
var (
	// ErrBoundaryVertexCount signals a ring with fewer than 3 or more than 50
	// distinct vertices (the closing repeat is not counted).
	ErrBoundaryVertexCount = errors.New("boundary must have between 3 and 50 distinct vertices")
	// ErrBoundaryDuplicateVertex signals two non-adjacent-by-closure vertices
	// with identical coordinates.
	ErrBoundaryDuplicateVertex = errors.New("boundary vertices must be distinct")
	// ErrBoundarySelfIntersecting signals two non-adjacent edges of the ring
	// crossing (the ring is not a simple polygon).
	ErrBoundarySelfIntersecting = errors.New("boundary must not self-intersect")
	// ErrBoundaryOutOfBounds signals a vertex outside the UK bounding box.
	ErrBoundaryOutOfBounds = errors.New("boundary vertices must be within the UK")
)

// WatchZone is a user's geofenced monitoring area scoped to one planning
// authority: either a circle (centre + radius) or, when Boundary is non-nil,
// a custom-shape polygon. A custom-shape zone still carries Latitude,
// Longitude and RadiusMetres — they hold the polygon's derived centroid and
// enclosing radius (see WithBoundary) so every existing circle-shaped read
// path (map centring, list rows, boundingBox, GDPR export) keeps working
// unchanged; Boundary == nil is the sole "this is a circle" discriminator.
// Exported fields keep it a plain Go value; the constructor enforces all
// invariants.
type WatchZone struct {
	ID                  string
	UserID              string
	Name                string
	Latitude            float64
	Longitude           float64
	RadiusMetres        float64
	AuthorityID         int
	CreatedAt           time.Time
	PushEnabled         bool
	EmailInstantEnabled bool
	Boundary            Boundary
}

// NewWatchZone validates and constructs a circle watch zone (Boundary nil).
// id, user id and name must be non-blank and radius and authority id must be
// positive. Coordinate range is deliberately NOT checked here — that is an
// HTTP-layer validation, so the domain accepts any latitude/longitude. This
// is an intentional asymmetry with custom-shape zones: NewBoundary DOES
// range-check its vertices against UK bounds, because a hand-drawn polygon
// with an out-of-range vertex is a client bug worth rejecting at construction
// (the shape is meaningless outside the UK), whereas a circle's centre is a
// single point whose plausibility is better judged with request context
// (e.g. postcode lookup) at the HTTP layer.
func NewWatchZone(id, userID, name string, latitude, longitude, radiusMetres float64, authorityID int, createdAt time.Time, pushEnabled, emailInstantEnabled bool) (WatchZone, error) {
	if strings.TrimSpace(id) == "" {
		return WatchZone{}, errors.New("id is required")
	}
	if strings.TrimSpace(userID) == "" {
		return WatchZone{}, errors.New("user id is required")
	}
	if strings.TrimSpace(name) == "" {
		return WatchZone{}, errors.New("name is required")
	}
	if radiusMetres <= 0 {
		return WatchZone{}, errors.New("radius must be positive")
	}
	if authorityID <= 0 {
		return WatchZone{}, errors.New("authority id must be positive")
	}
	return WatchZone{
		ID:                  id,
		UserID:              userID,
		Name:                name,
		Latitude:            latitude,
		Longitude:           longitude,
		RadiusMetres:        radiusMetres,
		AuthorityID:         authorityID,
		CreatedAt:           createdAt,
		PushEnabled:         pushEnabled,
		EmailInstantEnabled: emailInstantEnabled,
	}, nil
}

// metresPerDegreeLat is the approximate number of metres in one degree of
// latitude. It is treated as a constant: the meridional variation is sub-1% and
// irrelevant to a coarse bounding-box prune. One degree of longitude shrinks by
// cos(latitude), so the east-west offset is scaled by it.
const metresPerDegreeLat = 111320

// boundingBox returns the axis-aligned latitude/longitude box that circumscribes
// the zone's circle, derived from the centre and radius. It is the index-served
// prune for the notify-path containment query (store_cosmos.go): a candidate
// point outside the box cannot be inside the circle, and the exact ST_DISTANCE
// residual rejects the box corners that fall outside the circle. UK-only — no
// antimeridian or pole wrap is needed.
func (z WatchZone) boundingBox() (minLat, maxLat, minLon, maxLon float64) {
	dLat := z.RadiusMetres / metresPerDegreeLat
	latRadians := z.Latitude * math.Pi / 180
	dLon := z.RadiusMetres / (metresPerDegreeLat * math.Cos(latRadians))
	return z.Latitude - dLat, z.Latitude + dLat, z.Longitude - dLon, z.Longitude + dLon
}

// IsCustomShape reports whether z is a custom-shape (polygon) zone rather
// than a circle. Boundary == nil (the zero value) is the sole discriminator.
func (z WatchZone) IsCustomShape() bool {
	return len(z.Boundary) > 0
}

// WithBoundary returns a copy of z converted to a custom-shape zone: vertices
// is validated and normalised via NewBoundary, and Latitude, Longitude and
// RadiusMetres are recomputed as the polygon's centroid and enclosing radius.
// Deriving those three fields keeps every existing circle-shaped read path
// working unchanged against a custom-shape zone underneath.
func (z WatchZone) WithBoundary(vertices []Coordinate) (WatchZone, error) {
	b, err := NewBoundary(vertices)
	if err != nil {
		return WatchZone{}, err
	}
	lat, lon := b.Centroid()
	updated, err := NewWatchZone(
		z.ID, z.UserID, z.Name,
		lat, lon, b.EnclosingRadiusMetres(),
		z.AuthorityID, z.CreatedAt,
		z.PushEnabled, z.EmailInstantEnabled,
	)
	if err != nil {
		return WatchZone{}, err
	}
	updated.Boundary = b
	return updated, nil
}

// Coordinate is a single lon/lat point, GeoJSON-ordered (longitude first) to
// match the coordinate order the store and HTTP layers exchange with clients.
type Coordinate struct {
	Longitude float64
	Latitude  float64
}

// Boundary is a closed polygon ring for a custom-shape watch zone. A nil
// Boundary means the zone is a circle. Once constructed via NewBoundary, the
// last vertex always equals the first (a closed ring); NewBoundary is the
// only supported way to obtain a valid, validated Boundary.
type Boundary []Coordinate

const (
	// minBoundaryVertices and maxBoundaryVertices bound the ring size,
	// excluding the closing repeat: 3 is the geometric minimum for a
	// polygon, 50 is generous for a hand-drawn shape while bounding payload
	// size and the cost of the ST_Covers matching query.
	minBoundaryVertices = 3
	maxBoundaryVertices = 50

	// ukMinLatitude, ukMaxLatitude, ukMinLongitude and ukMaxLongitude are a
	// generous bounding box around Great Britain and Northern Ireland, used
	// only to reject boundary vertices that are obviously not the UK — not a
	// precise coastline check.
	ukMinLatitude  = 49.5
	ukMaxLatitude  = 61.0
	ukMinLongitude = -8.5
	ukMaxLongitude = 2.0
)

// NewBoundary validates and normalises raw vertices into a closed polygon
// ring: 3-50 distinct vertices (after auto-closing, not counting the closing
// repeat), an open ring is auto-closed by appending a copy of the first
// vertex (a ring that already closes is left as-is), all vertices within UK
// bounds, and the ring must be simple (no two non-adjacent edges cross).
func NewBoundary(vertices []Coordinate) (Boundary, error) {
	if len(vertices) == 0 {
		return nil, ErrBoundaryVertexCount
	}
	ring := make(Boundary, len(vertices))
	copy(ring, vertices)
	if ring[0] != ring[len(ring)-1] {
		ring = append(ring, ring[0])
	}
	distinct := ring[:len(ring)-1]
	if len(distinct) < minBoundaryVertices || len(distinct) > maxBoundaryVertices {
		return nil, ErrBoundaryVertexCount
	}
	seen := make(map[Coordinate]struct{}, len(distinct))
	for _, v := range distinct {
		if _, ok := seen[v]; ok {
			return nil, ErrBoundaryDuplicateVertex
		}
		seen[v] = struct{}{}
	}
	for _, v := range distinct {
		if v.Latitude < ukMinLatitude || v.Latitude > ukMaxLatitude ||
			v.Longitude < ukMinLongitude || v.Longitude > ukMaxLongitude {
			return nil, ErrBoundaryOutOfBounds
		}
	}
	if ring.selfIntersects() {
		return nil, ErrBoundarySelfIntersecting
	}
	return ring, nil
}

// selfIntersects reports whether any two non-adjacent edges of the closed
// ring cross. O(n²) over the edges; n is capped at 50 vertices by NewBoundary
// so this is cheap and a sweep-line algorithm is not warranted.
func (b Boundary) selfIntersects() bool {
	n := len(b) - 1 // number of edges (== number of distinct vertices) in the closed ring
	if n < 3 {
		return false
	}
	for i := range n {
		a1, a2 := b[i], b[i+1]
		for j := i + 2; j < n; j++ {
			if i == 0 && j == n-1 {
				// Edge n-1 (b[n-1]-b[n]) and edge 0 (b[0]-b[1]) share vertex
				// b[0]==b[n]: adjacent via the ring's wraparound, not a crossing.
				continue
			}
			b1, b2 := b[j], b[j+1]
			if segmentsIntersect(a1, a2, b1, b2) {
				return true
			}
		}
	}
	return false
}

// segmentsIntersect reports whether segments p1-p2 and p3-p4 intersect,
// including one touching the other at an endpoint or lying collinearly on
// it. Standard orientation-based test (see e.g. CLRS §33.1).
func segmentsIntersect(p1, p2, p3, p4 Coordinate) bool {
	d1 := orientation(p3, p4, p1)
	d2 := orientation(p3, p4, p2)
	d3 := orientation(p1, p2, p3)
	d4 := orientation(p1, p2, p4)

	if ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) &&
		((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0)) {
		return true
	}
	if d1 == 0 && onSegment(p3, p4, p1) {
		return true
	}
	if d2 == 0 && onSegment(p3, p4, p2) {
		return true
	}
	if d3 == 0 && onSegment(p1, p2, p3) {
		return true
	}
	if d4 == 0 && onSegment(p1, p2, p4) {
		return true
	}
	return false
}

// orientation returns (twice) the signed area of triangle a,b,c: positive for
// counter-clockwise, negative for clockwise, zero when the three points are
// collinear.
func orientation(a, b, c Coordinate) float64 {
	return (b.Longitude-a.Longitude)*(c.Latitude-a.Latitude) -
		(b.Latitude-a.Latitude)*(c.Longitude-a.Longitude)
}

// onSegment reports whether point p, already known to be collinear with
// segment a-b, lies within its bounding box — i.e. actually on the segment
// rather than on the line through it extended beyond either endpoint.
func onSegment(a, b, p Coordinate) bool {
	return p.Longitude >= math.Min(a.Longitude, b.Longitude) && p.Longitude <= math.Max(a.Longitude, b.Longitude) &&
		p.Latitude >= math.Min(a.Latitude, b.Latitude) && p.Latitude <= math.Max(a.Latitude, b.Latitude)
}

// earthRadiusMetres is the mean Earth radius used by haversineMetres.
const earthRadiusMetres = 6371000.0

// haversineMetres returns the great-circle distance between two lat/lon
// points, in metres.
func haversineMetres(lat1, lon1, lat2, lon2 float64) float64 {
	const degToRad = math.Pi / 180
	phi1, phi2 := lat1*degToRad, lat2*degToRad
	dPhi := (lat2 - lat1) * degToRad
	dLambda := (lon2 - lon1) * degToRad
	a := math.Sin(dPhi/2)*math.Sin(dPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*math.Sin(dLambda/2)*math.Sin(dLambda/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMetres * c
}

// Centroid returns the polygon centroid of the boundary ring: the
// area-weighted (shoelace-formula) centroid, not the arithmetic mean of the
// vertices — the two differ for any non-uniform shape (e.g. an L-shape).
// Longitude and latitude are treated as planar x/y, an acceptable
// approximation at the sub-tens-of-km scale a hand-drawn watch zone can
// reach.
func (b Boundary) Centroid() (latitude, longitude float64) {
	n := len(b) - 1 // closed ring: last vertex duplicates the first
	if n < 1 {
		return 0, 0
	}
	var area, cx, cy float64
	for i := range n {
		p0, p1 := b[i], b[i+1]
		cross := p0.Longitude*p1.Latitude - p1.Longitude*p0.Latitude
		area += cross
		cx += (p0.Longitude + p1.Longitude) * cross
		cy += (p0.Latitude + p1.Latitude) * cross
	}
	area /= 2
	if area == 0 {
		// Degenerate (zero-area, e.g. collinear) ring: fall back to the
		// vertex average rather than dividing by zero.
		var sumLat, sumLon float64
		for _, v := range b[:n] {
			sumLat += v.Latitude
			sumLon += v.Longitude
		}
		return sumLat / float64(n), sumLon / float64(n)
	}
	cx /= 6 * area
	cy /= 6 * area
	return cy, cx
}

// EnclosingRadiusMetres returns the greatest great-circle distance from the
// boundary's centroid to any vertex, in metres: the radius of the smallest
// centroid-centred circle that fully encloses the shape. This, together with
// the centroid, is what a polygon zone derives into the existing circle
// columns (see WithBoundary) so every read path built for circles keeps
// working unchanged.
func (b Boundary) EnclosingRadiusMetres() float64 {
	lat, lon := b.Centroid()
	n := len(b) - 1
	var maxDistance float64
	for _, v := range b[:n] {
		if d := haversineMetres(lat, lon, v.Latitude, v.Longitude); d > maxDistance {
			maxDistance = d
		}
	}
	return maxDistance
}

// ZoneUpdate is a partial PATCH: a nil field leaves the existing value
// untouched.
//
// Boundary is tri-state, which a plain pointer-to-value cannot express, so it
// is a pointer to the slice type instead:
//   - nil                          — absent: leave the zone's shape alone.
//   - non-nil, pointing to len-0   — explicit null: convert back to a circle,
//     keeping the zone's existing centre and radius.
//   - non-nil, pointing to len>0  — set this custom shape: the centre and
//     radius are recomputed from it via NewBoundary.
//
// This is the domain-level representation of the tri-state; decoding the
// HTTP PATCH body's `"boundary": null` vs absent vs a value into this shape
// is a separate, JSON-layer concern (json.RawMessage) handled by the handler.
type ZoneUpdate struct {
	Name                *string
	Latitude            *float64
	Longitude           *float64
	RadiusMetres        *float64
	AuthorityID         *int
	PushEnabled         *bool
	EmailInstantEnabled *bool
	Boundary            *Boundary
}

// WithUpdates returns a copy of the zone with the non-nil fields of u applied,
// re-validated through the constructor — so a merge that would violate an
// invariant (e.g. a blank name) returns an error rather than a corrupt zone.
// Identity (id, user id) and the creation timestamp are immutable across an
// update. See ZoneUpdate.Boundary for the tri-state shape semantics.
func (z WatchZone) WithUpdates(u ZoneUpdate) (WatchZone, error) {
	updated := z
	if u.Name != nil {
		updated.Name = *u.Name
	}
	if u.Latitude != nil {
		updated.Latitude = *u.Latitude
	}
	if u.Longitude != nil {
		updated.Longitude = *u.Longitude
	}
	if u.RadiusMetres != nil {
		updated.RadiusMetres = *u.RadiusMetres
	}
	if u.AuthorityID != nil {
		updated.AuthorityID = *u.AuthorityID
	}
	if u.PushEnabled != nil {
		updated.PushEnabled = *u.PushEnabled
	}
	if u.EmailInstantEnabled != nil {
		updated.EmailInstantEnabled = *u.EmailInstantEnabled
	}

	if u.Boundary != nil {
		if len(*u.Boundary) == 0 {
			// Explicit null: revert to a circle, keeping whatever centre and
			// radius the merge above has settled on (the zone's existing
			// derived values, unless this same update also set them).
			updated.Boundary = nil
		} else {
			b, err := NewBoundary(*u.Boundary)
			if err != nil {
				return WatchZone{}, err
			}
			lat, lon := b.Centroid()
			updated.Boundary = b
			updated.Latitude = lat
			updated.Longitude = lon
			updated.RadiusMetres = b.EnclosingRadiusMetres()
		}
	}

	result, err := NewWatchZone(
		updated.ID,
		updated.UserID,
		updated.Name,
		updated.Latitude,
		updated.Longitude,
		updated.RadiusMetres,
		updated.AuthorityID,
		updated.CreatedAt,
		updated.PushEnabled,
		updated.EmailInstantEnabled,
	)
	if err != nil {
		return WatchZone{}, err
	}
	result.Boundary = updated.Boundary
	return result, nil
}
