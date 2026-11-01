import { describe, it, expect } from 'vitest';
import {
  LARGE_ZONE_AREA_M2,
  boundaryAreaM2,
  isLargeBoundary,
  isLargeCircle,
  isLargeZone,
} from '../largeZone';
import type { WatchZoneBoundary } from '../types';

function squareBoundary(
  centreLat: number,
  centreLng: number,
  sizeDegrees: number,
): WatchZoneBoundary {
  const h = sizeDegrees / 2;
  const ring: [number, number][] = [
    [centreLng - h, centreLat - h],
    [centreLng + h, centreLat - h],
    [centreLng + h, centreLat + h],
    [centreLng - h, centreLat + h],
    [centreLng - h, centreLat - h],
  ];
  return { type: 'Polygon', coordinates: [ring] };
}

describe('largeZone', () => {
  it('uses the area of a 2 km circle as the threshold', () => {
    expect(LARGE_ZONE_AREA_M2).toBeCloseTo(12_566_371, -1);
  });

  it('does not treat a 2000 m circle as large', () => {
    expect(isLargeCircle(2000)).toBe(false);
  });

  it('treats a 2001 m circle as large', () => {
    expect(isLargeCircle(2001)).toBe(true);
  });

  it('treats a 0.1 degree square at 52N as large', () => {
    expect(isLargeBoundary(squareBoundary(52, -1, 0.1))).toBe(true);
  });

  it('does not treat a small polygon under 12.57 km2 as large', () => {
    expect(isLargeBoundary(squareBoundary(52, -1, 0.03))).toBe(false);
  });

  it('computes the area of a 0.1 degree square at 52N within 1% of 76.12 km2', () => {
    const area = boundaryAreaM2(squareBoundary(52, -1, 0.1));
    expect(Math.abs(area - 76_120_000) / 76_120_000).toBeLessThan(0.01);
  });

  it('is not large for a degenerate boundary', () => {
    expect(isLargeBoundary({ type: 'Polygon', coordinates: [] })).toBe(false);
    expect(
      isLargeBoundary({
        type: 'Polygon',
        coordinates: [
          [
            [0, 0],
            [1, 1],
          ],
        ],
      }),
    ).toBe(false);
  });

  it('is independent of ring winding direction', () => {
    const b = squareBoundary(52, -1, 0.1);
    const reversed: WatchZoneBoundary = {
      type: 'Polygon',
      coordinates: [[...(b.coordinates[0] ?? [])].reverse()],
    };
    expect(isLargeBoundary(reversed)).toBe(true);
  });

  it('isLargeZone uses the boundary when present, not the radius', () => {
    expect(
      isLargeZone({ radiusMetres: 10000, boundary: squareBoundary(52, -1, 0.03) }),
    ).toBe(false);
  });

  it('isLargeZone uses the radius when there is no boundary', () => {
    expect(isLargeZone({ radiusMetres: 5000, boundary: null })).toBe(true);
    expect(isLargeZone({ radiusMetres: 1000, boundary: null })).toBe(false);
  });
});
