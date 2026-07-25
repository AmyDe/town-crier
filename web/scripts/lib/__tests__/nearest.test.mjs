import { describe, it, expect } from 'vitest';
import { haversineDistanceKm, nearestK, centroidOf } from '../nearest.mjs';

describe('haversineDistanceKm', () => {
  it('matches a hand-computed value for a known one-degree-of-latitude pair', () => {
    // At the same longitude, haversine reduces exactly to R * dLat(radians) —
    // an analytically exact case, not just "looks about right". One degree of
    // latitude at the mean Earth radius (6371 km) is
    // 6371 * (Math.PI / 180) km ≈ 111.1949 km.
    const a = { lat: 51.0, lng: -1.0 };
    const b = { lat: 52.0, lng: -1.0 };
    const expectedKm = 6371 * (Math.PI / 180);
    expect(haversineDistanceKm(a, b)).toBeCloseTo(expectedKm, 6);
    expect(haversineDistanceKm(a, b)).toBeCloseTo(111.1949, 3);
  });

  it('is zero for two identical points', () => {
    const p = { lat: 50.2632, lng: -5.051 };
    expect(haversineDistanceKm(p, p)).toBe(0);
  });

  it('is symmetric: distance(a, b) === distance(b, a)', () => {
    const a = { lat: 50.2632, lng: -5.051 };
    const b = { lat: 51.4545, lng: -2.5879 };
    expect(haversineDistanceKm(a, b)).toBeCloseTo(haversineDistanceKm(b, a), 10);
  });
});

describe('nearestK', () => {
  const identity = (c) => c.slug;

  it('returns the K nearest candidates in ascending distance order', () => {
    const origin = { slug: 'origin', lat: 51.0, lng: -1.0 };
    const near = { slug: 'near', lat: 51.01, lng: -1.0 };
    const mid = { slug: 'mid', lat: 51.5, lng: -1.0 };
    const far = { slug: 'far', lat: 55.0, lng: -1.0 };
    const result = nearestK(origin, [far, mid, near], 2, { identity });
    expect(result.map((r) => r.slug)).toEqual(['near', 'mid']);
  });

  it('excludes the origin from its own candidate pool by identity, not by bare slug alone', () => {
    // Two towns with the SAME bare slug in different authorities (e.g. two
    // "Richmond"s) must never wrongly exclude each other — only an exact
    // identity match (here, authoritySlug/slug) is a genuine self-exclusion.
    const origin = { slug: 'richmond', authoritySlug: 'yorkshire', lat: 54.4, lng: -1.7 };
    const otherRichmond = {
      slug: 'richmond',
      authoritySlug: 'richmond-upon-thames',
      lat: 51.46,
      lng: -0.3,
    };
    const compositeIdentity = (c) => `${c.authoritySlug}/${c.slug}`;

    const result = nearestK(origin, [otherRichmond], 8, { identity: compositeIdentity });

    // The other Richmond is NOT excluded — it has a different composite identity.
    expect(result).toHaveLength(1);
    expect(result[0]).toBe(otherRichmond);
  });

  it('excludes the origin itself when it appears in the candidate pool', () => {
    const origin = { slug: 'truro', lat: 50.2632, lng: -5.051 };
    const other = { slug: 'falmouth', lat: 50.1526, lng: -5.0745 };
    const result = nearestK(origin, [origin, other], 8, { identity });
    expect(result).toEqual([other]);
  });

  it('breaks exact distance ties by tieBreakKey (slug) ascending', () => {
    const origin = { slug: 'origin', lat: 51.0, lng: -1.0 };
    // Two candidates at an IDENTICAL distance from origin (symmetric offset).
    const zebra = { slug: 'zebra', lat: 51.1, lng: -1.0 };
    const alpha = { slug: 'alpha', lat: 50.9, lng: -1.0 };
    const result = nearestK(origin, [zebra, alpha], 2, {
      identity,
      tieBreakKey: (c) => c.slug,
    });
    expect(
      haversineDistanceKm(origin, zebra) === haversineDistanceKm(origin, alpha),
    ).toBe(true);
    expect(result.map((r) => r.slug)).toEqual(['alpha', 'zebra']);
  });

  it('never relies on Array.prototype.sort stability alone: reversing input order does not change tie-break output', () => {
    const origin = { slug: 'origin', lat: 51.0, lng: -1.0 };
    const zebra = { slug: 'zebra', lat: 51.1, lng: -1.0 };
    const alpha = { slug: 'alpha', lat: 50.9, lng: -1.0 };
    const forward = nearestK(origin, [zebra, alpha], 2, { identity });
    const reversed = nearestK(origin, [alpha, zebra], 2, { identity });
    expect(forward.map((r) => r.slug)).toEqual(reversed.map((r) => r.slug));
    expect(forward.map((r) => r.slug)).toEqual(['alpha', 'zebra']);
  });

  it('never drops a valid candidate the caller supplied (does no filtering of its own beyond self-exclusion)', () => {
    // The function must not silently exclude an "unpublished-looking" entry —
    // filtering to only published/eligible candidates is the CALLER's job.
    // Every distinct entry handed in (other than the origin) comes back.
    const origin = { slug: 'origin', lat: 51.0, lng: -1.0 };
    const candidates = Array.from({ length: 5 }, (_, i) => ({
      slug: `c${i}`,
      lat: 51.0 + i * 0.1,
      lng: -1.0,
    }));
    const result = nearestK(origin, candidates, 10, { identity });
    expect(result).toHaveLength(5);
    expect(new Set(result.map((r) => r.slug))).toEqual(
      new Set(candidates.map((c) => c.slug)),
    );
  });

  it('returns every available candidate, unpadded, when K exceeds the candidate pool', () => {
    const origin = { slug: 'origin', lat: 51.0, lng: -1.0 };
    const a = { slug: 'a', lat: 51.1, lng: -1.0 };
    const b = { slug: 'b', lat: 51.2, lng: -1.0 };
    const result = nearestK(origin, [a, b], 6, { identity });
    expect(result).toHaveLength(2);
    expect(result.map((r) => r.slug)).toEqual(['a', 'b']);
  });

  it('returns an empty array when there are no candidates other than the origin', () => {
    const origin = { slug: 'origin', lat: 51.0, lng: -1.0 };
    expect(nearestK(origin, [origin], 8, { identity })).toEqual([]);
    expect(nearestK(origin, [], 8, { identity })).toEqual([]);
  });
});

describe('centroidOf', () => {
  it('is the arithmetic mean of the given points', () => {
    const points = [
      { lat: 50.0, lng: -5.0 },
      { lat: 52.0, lng: -3.0 },
    ];
    expect(centroidOf(points)).toEqual({ lat: 51.0, lng: -4.0 });
  });

  it('returns the point itself for a single-point list', () => {
    const point = { lat: 50.2632, lng: -5.051 };
    expect(centroidOf([point])).toEqual({ lat: 50.2632, lng: -5.051 });
  });

  it('returns null for an empty list (no honest centroid to report)', () => {
    expect(centroidOf([])).toBeNull();
  });
});
