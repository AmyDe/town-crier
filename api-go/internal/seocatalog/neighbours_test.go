package seocatalog

import (
	"math"
	"slices"
	"testing"
)

func TestHaversineDistanceKm_KnownDistance(t *testing.T) {
	t.Parallel()
	london := geoPoint{lat: 51.5074, lng: -0.1278}
	paris := geoPoint{lat: 48.8566, lng: 2.3522}
	got := haversineDistanceKm(london, paris)
	if math.Abs(got-343.5) > 1.5 {
		t.Errorf("London-Paris = %.1f km, want about 343.5", got)
	}
}

func TestHaversineDistanceKm_IdenticalPointsIsZero(t *testing.T) {
	t.Parallel()
	p := geoPoint{lat: 54.2, lng: -1.1}
	if got := haversineDistanceKm(p, p); got != 0 {
		t.Errorf("distance = %v, want 0", got)
	}
}

func candidate(index int, identity, tieKey string, lat float64) neighbourCandidate {
	return neighbourCandidate{point: geoPoint{lat: lat, lng: 0}, identity: identity, tieKey: tieKey, index: index}
}

func TestNearestK_OrdersByDistanceExcludesSelfAndLimits(t *testing.T) {
	t.Parallel()
	pool := []neighbourCandidate{
		candidate(0, "origin", "origin", 50),
		candidate(1, "far", "far", 53),
		candidate(2, "near", "near", 50.1),
		candidate(3, "mid", "mid", 51),
	}
	got := nearestK(pool[0], pool, 2)
	if want := []int{2, 3}; !slices.Equal(got, want) {
		t.Errorf("nearestK = %v, want %v", got, want)
	}
}

func TestNearestK_TiesBreakByKeyThenIdentity(t *testing.T) {
	t.Parallel()
	pool := []neighbourCandidate{
		candidate(0, "origin", "origin", 50),
		candidate(1, "x/zeta", "zeta", 51),
		candidate(2, "y/alpha", "alpha", 51),
		candidate(3, "x/alpha", "alpha", 51),
	}
	got := nearestK(pool[0], pool, 3)
	if want := []int{3, 2, 1}; !slices.Equal(got, want) {
		t.Errorf("nearestK = %v, want %v", got, want)
	}
}

func TestNearestK_SameSlugDifferentIdentityIsNotSelf(t *testing.T) {
	t.Parallel()
	pool := []neighbourCandidate{
		candidate(0, "a/richmond", "richmond", 50),
		candidate(1, "b/richmond", "richmond", 51),
	}
	if got := nearestK(pool[0], pool, 8); !slices.Equal(got, []int{1}) {
		t.Errorf("nearestK = %v, want [1]", got)
	}
}

func TestCentroidOf(t *testing.T) {
	t.Parallel()
	if _, ok := centroidOf(nil); ok {
		t.Error("empty input must have no centroid")
	}
	c, ok := centroidOf([]geoPoint{{lat: 1, lng: 2}, {lat: 3, lng: 6}})
	if !ok || c.lat != 2 || c.lng != 4 {
		t.Errorf("centroid = %+v (ok %v), want {2 4}", c, ok)
	}
}
