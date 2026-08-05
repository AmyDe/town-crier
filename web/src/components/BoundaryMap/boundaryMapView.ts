import type { Coordinates } from '../../domain/types';

/**
 * Computes the map view `BoundaryMap` shows on first mount (GH#1031, bead
 * tc-7se1w.7): a fixed close-in view centred on the postcode-geocoded centre
 * when there are no vertices yet (a brand-new custom shape — today's
 * behaviour, unchanged), or a view that fits the bounding box of the
 * existing vertices with margin when re-opening a zone that already has a
 * drawn polygon. Without this, the fixed zoom clips a large polygon's
 * vertices/edges outside the visible map on reopen.
 *
 * Conceptual sibling of iOS's `BoundaryDrawingRegion.fitting` (tc-7se1w.2):
 * same shape of fix (pure, unit-testable bounds computation; 30% margin;
 * a minimum-span floor), expressed idiomatically for react-leaflet's
 * `bounds` prop instead of MapKit's `MKCoordinateRegion`.
 */

/** The fixed zoom used when there are no vertices yet — matches the zoom
 * `BoundaryMap` always used before tc-7se1w.7. */
export const FRESH_DRAW_ZOOM = 15;

/** Extra room around the tightest bounding box of the vertices, as a
 * fraction of the box's own span — keeps drawn edges/vertices from sitting
 * flush against the map's edge rather than comfortably inside it. */
export const BOUNDS_MARGIN_FRACTION = 0.3;

/** Smallest span (degrees of latitude/longitude) the fitted bounds are
 * allowed to shrink to — stops a single vertex or a tightly clustered few
 * vertices from producing an absurdly close-in zoom. */
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
