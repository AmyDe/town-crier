/**
 * Pure geographic helpers for the `/planning` cross-link mesh (GH #990 slices
 * 3+4, tc-gyw1q): haversine distance between two WGS84 points, deterministic
 * nearest-K selection, and an authority centroid (the arithmetic mean of its
 * own published towns). No I/O, no framework code, no committed data of its
 * own — every caller (`prerender-planning.mjs`) owns the published-set
 * filtering, gating, and page writes; these functions only ever see the exact
 * candidate pool they are handed.
 */

/**
 * Mean Earth radius in kilometres. The exact constant only affects the
 * absolute distance value, never the RANKING — every candidate in a given
 * call is measured with the same radius, so nearest-K ordering is unaffected
 * by which reasonable value is chosen here.
 * @type {number}
 */
const EARTH_RADIUS_KM = 6371;

/**
 * @typedef {Object} GeoPoint
 * @property {number} lat WGS84 latitude, degrees
 * @property {number} lng WGS84 longitude, degrees
 */

/**
 * @param {number} degrees
 * @returns {number} radians
 */
function toRadians(degrees) {
  return (degrees * Math.PI) / 180;
}

/**
 * Great-circle distance between two WGS84 points, in kilometres (the
 * standard haversine formula over the mean Earth radius).
 *
 * @param {GeoPoint} a
 * @param {GeoPoint} b
 * @returns {number}
 */
export function haversineDistanceKm(a, b) {
  const dLat = toRadians(b.lat - a.lat);
  const dLng = toRadians(b.lng - a.lng);
  const lat1 = toRadians(a.lat);
  const lat2 = toRadians(b.lat);
  const sinDLat = Math.sin(dLat / 2);
  const sinDLng = Math.sin(dLng / 2);
  const h = sinDLat * sinDLat + Math.cos(lat1) * Math.cos(lat2) * sinDLng * sinDLng;
  // Clamp to 1 before asin: floating-point error can push h fractionally over 1
  // for two near-identical points, which would otherwise make asin return NaN.
  return 2 * EARTH_RADIUS_KM * Math.asin(Math.min(1, Math.sqrt(h)));
}

/**
 * Select the K nearest candidates to `origin`, deterministically.
 *
 * Self-exclusion never relies on a bare `slug` alone: two different towns in
 * different authorities can legitimately share one (e.g. two "Richmond"s —
 * Yorkshire and Richmond-upon-Thames), so the caller supplies
 * `options.identity`, a key unique across the WHOLE candidate pool (a town's
 * full page path, an authority's slug), and any candidate whose identity
 * matches the origin's is excluded from its own neighbour list.
 *
 * Ties in distance break by `options.tieBreakKey` ascending (the town/
 * authority slug, per GH #990's decision), then by `identity` ascending as a
 * final, fully-deterministic fallback for the rare case two candidates share
 * even that key (e.g. same-slug towns in different authorities, equidistant)
 * — never left to `Array.prototype.sort`'s stability alone.
 *
 * Returns every available candidate, unpadded, when fewer than `k` remain
 * after self-exclusion — never pads or fabricates entries. Does no filtering
 * of its own beyond self-exclusion: the candidate pool the caller passes in is
 * exactly the pool considered (a below-threshold/unpublished/gated-out entry
 * must never appear in `candidates` in the first place — that is the
 * caller's job, not this function's).
 *
 * @template {GeoPoint} T
 * @param {T} origin
 * @param {ReadonlyArray<T>} candidates the full candidate pool, already
 *   restricted by the caller to published/eligible entries
 * @param {number} k
 * @param {Object} options
 * @param {(candidate: T) => string} options.identity unique key across the
 *   whole pool, used for self-exclusion and the final tie-break
 * @param {(candidate: T) => string} [options.tieBreakKey] primary tie-break
 *   key on an exact distance tie; defaults to `identity`
 * @returns {T[]}
 */
export function nearestK(origin, candidates, k, options) {
  const { identity, tieBreakKey = identity } = options;
  const originIdentity = identity(origin);

  const scored = [];
  for (const candidate of candidates) {
    if (identity(candidate) === originIdentity) {
      continue;
    }
    scored.push({ candidate, distanceKm: haversineDistanceKm(origin, candidate) });
  }

  scored.sort((x, y) => {
    if (x.distanceKm !== y.distanceKm) {
      return x.distanceKm - y.distanceKm;
    }
    const xKey = tieBreakKey(x.candidate);
    const yKey = tieBreakKey(y.candidate);
    if (xKey !== yKey) {
      return xKey < yKey ? -1 : 1;
    }
    const xId = identity(x.candidate);
    const yId = identity(y.candidate);
    if (xId === yId) {
      return 0;
    }
    return xId < yId ? -1 : 1;
  });

  return scored.slice(0, k).map((s) => s.candidate);
}

/**
 * Arithmetic mean of a set of points — an authority's centroid, derived ONLY
 * from its own published towns (the caller filters; this does no filtering of
 * its own). Returns `null` for an empty list: an authority with zero
 * published towns has no honest centroid to report (GH #990 decision — the
 * caller omits the neighbours section rather than guessing one).
 *
 * @param {ReadonlyArray<GeoPoint>} points
 * @returns {GeoPoint | null}
 */
export function centroidOf(points) {
  if (points.length === 0) {
    return null;
  }
  let sumLat = 0;
  let sumLng = 0;
  for (const point of points) {
    sumLat += point.lat;
    sumLng += point.lng;
  }
  return { lat: sumLat / points.length, lng: sumLng / points.length };
}
