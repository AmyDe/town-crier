package applications

import (
	"math"
	"strconv"
	"strings"
)

// maxZoom is the inclusive upper bound on the standard slippy-map zoom range a
// client may request; baseGridDegrees is the grid cell size (in degrees) at
// zoom 0. Each cell is 1/8 of a zoom tile (a tile is 360/2^z degrees wide), so
// a screenful of a few tiles yields a bounded handful of cluster cells.
//
// This table (and the exported GridDegreesForZoom/FinestGridDegrees below)
// moved here from watchzones/nearby.go (GH#924): the zoom -> grid policy is a
// client-visible density contract shared by both the authed watch-zone map and
// the anonymous map (anonclusters.go) — duplicating it invites drift where the
// two maps cluster differently at the same zoom. watchzones already imports
// this package, so the dependency direction was already established.
const (
	maxZoom         = 20
	baseGridDegrees = 45.0 // 360 / 2^3 (eight cells per tile at zoom 0)
)

// zoomGridDegrees is the server-owned zoom -> grid-cell-size lookup. Index z
// holds the cell size in degrees for slippy zoom z: baseGridDegrees / 2^z, so
// the cell halves with every zoom level (45 deg at z=0 down to ~4.3e-5 deg at
// z=20, where each application is effectively its own cell). Keeping this
// table here lets density be retuned without touching a store or shipping a
// client release.
var zoomGridDegrees = func() [maxZoom + 1]float64 {
	var table [maxZoom + 1]float64
	for z := range table {
		table[z] = baseGridDegrees / float64(uint64(1)<<uint(z))
	}
	return table
}()

// GridDegreesForZoom resolves a slippy-map zoom level to its grid cell size in
// degrees via zoomGridDegrees. ok is false for a zoom outside [0, maxZoom] —
// there is no sensible "nearest legal zoom" to clamp an out-of-range value to,
// so callers turn a false ok into a clean 400 rather than guessing.
func GridDegreesForZoom(zoom int) (float64, bool) {
	if zoom < 0 || zoom > maxZoom {
		return 0, false
	}
	return zoomGridDegrees[zoom], true
}

// FinestGridDegrees returns the zoom-20 (finest) grid cell size in degrees:
// the coalesce threshold below which a multi-member cluster cell can never be
// split by further zooming, so it carries a capped applicationIds member list
// (see ClusterQuery.CoalesceThresholdDegrees). It is independent of the
// request's own zoom/grid size — every cluster query derives its coalesce
// threshold from this same finest cell, not from its own GridSizeDegrees.
func FinestGridDegrees() float64 {
	return zoomGridDegrees[maxZoom]
}

// BBox is a parsed ?bbox=west,south,east,north viewport rectangle in WGS84
// decimal degrees, as produced by ParseBBox.
type BBox struct {
	West, South, East, North float64
}

// ParseBBox resolves ?bbox=west,south,east,north into a BBox. It rejects
// (ok == false) anything that is not exactly four finite decimal degrees, with
// coordinates in range (lng [-180,180], lat [-90,90]) and strictly ordered
// (west < east, south < north), so a malformed viewport is a clean 400 rather
// than a degenerate or world-spanning query. Moved from watchzones/nearby.go's
// unexported parseBBox (GH#924) — byte-identical semantics, now shared.
func ParseBBox(raw string) (BBox, bool) {
	if raw == "" {
		return BBox{}, false
	}
	parts := strings.Split(raw, ",")
	if len(parts) != 4 {
		return BBox{}, false
	}
	vals := make([]float64, 4)
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return BBox{}, false
		}
		vals[i] = v
	}
	box := BBox{West: vals[0], South: vals[1], East: vals[2], North: vals[3]}
	if box.West < -180 || box.West > 180 || box.East < -180 || box.East > 180 {
		return BBox{}, false
	}
	if box.South < -90 || box.South > 90 || box.North < -90 || box.North > 90 {
		return BBox{}, false
	}
	if box.West >= box.East || box.South >= box.North {
		return BBox{}, false
	}
	return box, true
}
