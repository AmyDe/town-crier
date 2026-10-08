package seocatalog

import (
	"math"
	"sort"
)

const earthRadiusKm = 6371

type geoPoint struct {
	lat, lng float64
}

func toRadians(degrees float64) float64 { return degrees * math.Pi / 180 }

// haversineDistanceKm is the great-circle distance in kilometres over the mean
// Earth radius. Only the ordering matters to callers.
func haversineDistanceKm(a, b geoPoint) float64 {
	dLat := toRadians(b.lat - a.lat)
	dLng := toRadians(b.lng - a.lng)
	lat1 := toRadians(a.lat)
	lat2 := toRadians(b.lat)
	sinDLat := math.Sin(dLat / 2)
	sinDLng := math.Sin(dLng / 2)
	h := sinDLat*sinDLat + math.Cos(lat1)*math.Cos(lat2)*sinDLng*sinDLng
	// Clamped: rounding can push h fractionally over 1, making Asin return NaN.
	return 2 * earthRadiusKm * math.Asin(math.Min(1, math.Sqrt(h)))
}

// neighbourCandidate is a point with the keys nearestK needs: identity is
// unique across the whole pool, tieKey breaks exact distance ties first.
type neighbourCandidate struct {
	point    geoPoint
	identity string
	tieKey   string
	index    int
}

// nearestK returns the indexes (into the original slice) of the k candidates
// nearest to origin, excluding any candidate sharing the origin's identity.
// Ties break by tieKey, then identity.
func nearestK(origin neighbourCandidate, candidates []neighbourCandidate, k int) []int {
	type scored struct {
		c  neighbourCandidate
		km float64
	}
	var list []scored
	for _, c := range candidates {
		if c.identity == origin.identity {
			continue
		}
		list = append(list, scored{c: c, km: haversineDistanceKm(origin.point, c.point)})
	}
	sort.SliceStable(list, func(i, j int) bool {
		x, y := list[i], list[j]
		if x.km != y.km {
			return x.km < y.km
		}
		if x.c.tieKey != y.c.tieKey {
			return x.c.tieKey < y.c.tieKey
		}
		return x.c.identity < y.c.identity
	})
	if len(list) > k {
		list = list[:k]
	}
	out := make([]int, len(list))
	for i, s := range list {
		out[i] = s.c.index
	}
	return out
}

// centroidOf is the arithmetic mean of the points; ok is false when empty.
func centroidOf(points []geoPoint) (geoPoint, bool) {
	if len(points) == 0 {
		return geoPoint{}, false
	}
	var sumLat, sumLng float64
	for _, p := range points {
		sumLat += p.lat
		sumLng += p.lng
	}
	n := float64(len(points))
	return geoPoint{lat: sumLat / n, lng: sumLng / n}, true
}
