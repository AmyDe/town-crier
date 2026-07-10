package applications

import (
	"testing"
)

// TestGridDegreesForZoom pins the zoom -> grid-cell-size lookup this package
// now owns (moved from watchzones, GH#924): each cell halves per zoom level
// (baseGridDegrees / 2^z), and a zoom outside [0, maxZoom] is rejected.
func TestGridDegreesForZoom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		zoom int
		want float64
		ok   bool
	}{
		{"zoom 0 is the base grid", 0, 45.0, true},
		{"zoom 1 halves the base grid", 1, 22.5, true},
		{"zoom 20 is the finest grid", 20, 45.0 / float64(uint64(1)<<20), true},
		{"negative zoom is rejected", -1, 0, false},
		{"zoom above maxZoom is rejected", 21, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := GridDegreesForZoom(tc.zoom)
			if ok != tc.ok {
				t.Fatalf("GridDegreesForZoom(%d) ok = %v, want %v", tc.zoom, ok, tc.ok)
			}
			if ok && got != tc.want {
				t.Errorf("GridDegreesForZoom(%d) = %v, want %v", tc.zoom, got, tc.want)
			}
		})
	}
}

// TestFinestGridDegrees proves the coalesce-threshold helper always returns the
// zoom-20 cell size, independent of any request zoom.
func TestFinestGridDegrees(t *testing.T) {
	t.Parallel()
	want := 45.0 / float64(uint64(1)<<20)
	if got := FinestGridDegrees(); got != want {
		t.Errorf("FinestGridDegrees() = %v, want %v", got, want)
	}
}

// TestParseBBox mirrors the coverage watchzones' parseBBox previously had
// entirely through the HTTP handler, exercising the moved parser directly:
// well-formed rectangles parse, and malformed/out-of-range/degenerate input is
// rejected.
func TestParseBBox(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want BBox
		ok   bool
	}{
		{"valid rectangle", "-0.2,51.4,-0.05,51.6", BBox{West: -0.2, South: 51.4, East: -0.05, North: 51.6}, true},
		{"empty is rejected", "", BBox{}, false},
		{"wrong field count is rejected", "-0.2,51.4,-0.05", BBox{}, false},
		{"non-numeric field is rejected", "-0.2,51.4,x,51.6", BBox{}, false},
		{"NaN literal is rejected", "-0.2,51.4,NaN,51.6", BBox{}, false},
		{"Inf literal is rejected", "-0.2,51.4,Inf,51.6", BBox{}, false},
		{"west out of range is rejected", "-181,51.4,-0.05,51.6", BBox{}, false},
		{"east out of range is rejected", "-0.2,51.4,181,51.6", BBox{}, false},
		{"south out of range is rejected", "-0.2,-91,-0.05,51.6", BBox{}, false},
		{"north out of range is rejected", "-0.2,51.4,-0.05,91", BBox{}, false},
		{"west >= east is rejected", "-0.05,51.4,-0.2,51.6", BBox{}, false},
		{"south >= north is rejected", "-0.2,51.6,-0.05,51.4", BBox{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ParseBBox(tc.raw)
			if ok != tc.ok {
				t.Fatalf("ParseBBox(%q) ok = %v, want %v", tc.raw, ok, tc.ok)
			}
			if ok && got != tc.want {
				t.Errorf("ParseBBox(%q) = %+v, want %+v", tc.raw, got, tc.want)
			}
		})
	}
}
