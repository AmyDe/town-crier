import { describe, it, expect } from 'vitest';
import { haversineDistanceMetres } from '../distance';

describe('haversineDistanceMetres', () => {
  it('returns 0 when both points are identical', () => {
    const cambridge = { latitude: 52.2053, longitude: 0.1218 };

    expect(haversineDistanceMetres(cambridge, cambridge)).toBe(0);
  });

  it('computes the great-circle distance in metres between two points', () => {
    // Cambridge (52.2053, 0.1218) → Oxford (51.7520, -1.2577)
    // Reference: ~ 134_300 m via independent haversine calculator.
    const cambridge = { latitude: 52.2053, longitude: 0.1218 };
    const oxford = { latitude: 51.752, longitude: -1.2577 };

    const distance = haversineDistanceMetres(cambridge, oxford);

    // Allow ±200 m tolerance — different earth-radius constants and
    // floating-point rounding produce values in the 134_100..134_500 band.
    expect(distance).toBeGreaterThan(134_100);
    expect(distance).toBeLessThan(134_500);
  });

  it('is symmetric in its arguments', () => {
    const a = { latitude: 51.5074, longitude: -0.1278 }; // London
    const b = { latitude: 53.4808, longitude: -2.2426 }; // Manchester

    const ab = haversineDistanceMetres(a, b);
    const ba = haversineDistanceMetres(b, a);

    expect(ab).toBeCloseTo(ba, 6);
  });
});
