import type { Coordinates, WatchZoneBoundary, WatchZoneSummary } from './types';

const EARTH_RADIUS_M = 6_371_008.8;
const LARGE_ZONE_RADIUS_M = 2000;

/** Area of a 2 km circle. A zone is large only when strictly greater, so a free-tier 2 km circle never warns. */
export const LARGE_ZONE_AREA_M2 = Math.PI * LARGE_ZONE_RADIUS_M ** 2;

const toRadians = (degrees: number) => (degrees * Math.PI) / 180;

/** Area in m2 of the outer ring (`[longitude, latitude]` vertices), by shoelace on an equirectangular projection. */
export function boundaryAreaM2(boundary: WatchZoneBoundary): number {
  const ring = boundary.coordinates[0] ?? [];
  const last = ring[ring.length - 1];
  const first = ring[0];
  const closed = first && last && first[0] === last[0] && first[1] === last[1];
  const vertices = closed ? ring.slice(0, -1) : ring;
  if (vertices.length < 3) return 0;

  const meanLat = toRadians(vertices.reduce((sum, [, lat]) => sum + lat, 0) / vertices.length);
  const points = vertices.map(([lng, lat]) => ({
    x: toRadians(lng) * Math.cos(meanLat) * EARTH_RADIUS_M,
    y: toRadians(lat) * EARTH_RADIUS_M,
  }));

  let twiceArea = 0;
  points.forEach((p, i) => {
    const next = points[(i + 1) % points.length]!;
    twiceArea += p.x * next.y - next.x * p.y;
  });
  return Math.abs(twiceArea) / 2;
}

export function isLargeCircle(radiusMetres: number): boolean {
  return Math.PI * radiusMetres ** 2 > LARGE_ZONE_AREA_M2;
}

export function isLargeBoundary(boundary: WatchZoneBoundary): boolean {
  return boundaryAreaM2(boundary) > LARGE_ZONE_AREA_M2;
}

/** For a ring still being drawn: needs three vertices, and does not need the closing vertex. */
export function isLargeVertices(vertices: readonly Coordinates[]): boolean {
  const ring = vertices.map((v) => [v.longitude, v.latitude] as const);
  return isLargeBoundary({ type: 'Polygon', coordinates: [ring] });
}

/** Uses the boundary when the zone has one, else its radius. */
export function isLargeZone(
  zone: Pick<WatchZoneSummary, 'radiusMetres' | 'boundary'>,
): boolean {
  return zone.boundary ? isLargeBoundary(zone.boundary) : isLargeCircle(zone.radiusMetres);
}
