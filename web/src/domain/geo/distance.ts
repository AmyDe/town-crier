import type { Coordinates } from '../types';

const EARTH_RADIUS_METRES = 6_371_000;

/**
 * Great-circle distance between two points on Earth, in metres, computed via
 * the haversine formula. Mirrors the iOS sibling at
 * `mobile/ios/packages/town-crier-domain/Sources/ValueObjects/WatchZone.swift`
 * — same earth-radius constant, identical algorithm — so the two platforms
 * sort the Applications list in the same order for a given user (tc-ge7j /
 * tc-mso6).
 */
export function haversineDistanceMetres(
  a: Coordinates,
  b: Coordinates,
): number {
  const dLat = degreesToRadians(b.latitude - a.latitude);
  const dLon = degreesToRadians(b.longitude - a.longitude);
  const lat1 = degreesToRadians(a.latitude);
  const lat2 = degreesToRadians(b.latitude);

  const sinHalfDLat = Math.sin(dLat / 2);
  const sinHalfDLon = Math.sin(dLon / 2);
  const h =
    sinHalfDLat * sinHalfDLat +
    Math.cos(lat1) * Math.cos(lat2) * sinHalfDLon * sinHalfDLon;
  return 2 * EARTH_RADIUS_METRES * Math.asin(Math.sqrt(h));
}

function degreesToRadians(degrees: number): number {
  return (degrees * Math.PI) / 180;
}
