import type { Coordinates } from '../../domain/types';

/** The fixed zoom used when there are no vertices yet. */
export const FRESH_DRAW_ZOOM = 15;

/** Margin around the vertices' bounding box, as a fraction of its span. */
export const BOUNDS_MARGIN_FRACTION = 0.3;

/** Minimum fitted span in degrees, so one vertex or a tight cluster does not zoom in absurdly. */
export const MINIMUM_SPAN_DEGREES = 0.006;

export interface BoundaryMapFixedView {
  readonly kind: 'fixed';
  readonly center: Coordinates;
  readonly zoom: number;
}

export interface BoundaryMapFitBoundsView {
  readonly kind: 'fit-bounds';
  readonly southWest: Coordinates;
  readonly northEast: Coordinates;
}

export type BoundaryMapView = BoundaryMapFixedView | BoundaryMapFitBoundsView;

/**
 * Initial view for `BoundaryMap`: the fixed zoom at `initialCentre` when there
 * are no vertices, otherwise bounds fitting the vertices with margin so a large
 * re-opened polygon is not clipped.
 */
export function computeBoundaryMapView(
  vertices: readonly Coordinates[],
  initialCentre: Coordinates,
): BoundaryMapView {
  if (vertices.length === 0) {
    return { kind: 'fixed', center: initialCentre, zoom: FRESH_DRAW_ZOOM };
  }

  const latitudes = vertices.map((vertex) => vertex.latitude);
  const longitudes = vertices.map((vertex) => vertex.longitude);
  const minLatitude = Math.min(...latitudes);
  const maxLatitude = Math.max(...latitudes);
  const minLongitude = Math.min(...longitudes);
  const maxLongitude = Math.max(...longitudes);

  const centreLatitude = (minLatitude + maxLatitude) / 2;
  const centreLongitude = (minLongitude + maxLongitude) / 2;

  const latitudeSpan = Math.max(
    (maxLatitude - minLatitude) * (1 + BOUNDS_MARGIN_FRACTION),
    MINIMUM_SPAN_DEGREES,
  );
  const longitudeSpan = Math.max(
    (maxLongitude - minLongitude) * (1 + BOUNDS_MARGIN_FRACTION),
    MINIMUM_SPAN_DEGREES,
  );

  return {
    kind: 'fit-bounds',
    southWest: {
      latitude: centreLatitude - latitudeSpan / 2,
      longitude: centreLongitude - longitudeSpan / 2,
    },
    northEast: {
      latitude: centreLatitude + latitudeSpan / 2,
      longitude: centreLongitude + longitudeSpan / 2,
    },
  };
}
