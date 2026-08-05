import { describe, it, expect } from 'vitest';
import {
  computeBoundaryMapView,
  FRESH_DRAW_ZOOM,
  BOUNDS_MARGIN_FRACTION,
  MINIMUM_SPAN_DEGREES,
} from '../boundaryMapView';

describe('computeBoundaryMapView', () => {
  const initialCentre = { latitude: 52.2053, longitude: 0.1218 };

  it('returns a fixed view centred on the initial centre when there are no vertices yet', () => {
    const view = computeBoundaryMapView([], initialCentre);

    expect(view).toEqual({ kind: 'fixed', center: initialCentre, zoom: FRESH_DRAW_ZOOM });
  });

  it('fits the bounding box of the vertices, with every vertex strictly inside the bounds', () => {
    // A ~1.1km x 1.1km box — big enough that a fixed close-in zoom would clip it.
    const vertices = [
      { latitude: 51.5, longitude: -0.1 },
      { latitude: 51.51, longitude: -0.1 },
      { latitude: 51.51, longitude: -0.09 },
      { latitude: 51.5, longitude: -0.09 },
    ];

    const view = computeBoundaryMapView(vertices, { latitude: 51.505, longitude: -0.095 });

    if (view.kind !== 'fit-bounds') {
      throw new Error('expected a fit-bounds view');
    }
    for (const vertex of vertices) {
      expect(vertex.latitude).toBeGreaterThan(view.southWest.latitude);
      expect(vertex.latitude).toBeLessThan(view.northEast.latitude);
      expect(vertex.longitude).toBeGreaterThan(view.southWest.longitude);
      expect(vertex.longitude).toBeLessThan(view.northEast.longitude);
    }
  });

  it('centres the bounds on the vertices bounding box, not the initial centre', () => {
    const vertices = [
      { latitude: 51.5, longitude: -0.1 },
      { latitude: 51.51, longitude: -0.1 },
      { latitude: 51.51, longitude: -0.09 },
      { latitude: 51.5, longitude: -0.09 },
    ];

    const view = computeBoundaryMapView(vertices, { latitude: 51.6, longitude: -0.2 });

    if (view.kind !== 'fit-bounds') {
      throw new Error('expected a fit-bounds view');
    }
    const centreLatitude = (view.southWest.latitude + view.northEast.latitude) / 2;
    const centreLongitude = (view.southWest.longitude + view.northEast.longitude) / 2;
    expect(centreLatitude).toBeCloseTo(51.505, 5);
    expect(centreLongitude).toBeCloseTo(-0.095, 5);
  });

  it('applies the expected margin fraction to the raw bounding box span, not merely exceeding it', () => {
    const vertices = [
      { latitude: 51.5, longitude: -0.1 },
      { latitude: 51.51, longitude: -0.1 },
      { latitude: 51.51, longitude: -0.09 },
      { latitude: 51.5, longitude: -0.09 },
    ];

    const view = computeBoundaryMapView(vertices, { latitude: 51.505, longitude: -0.095 });

    if (view.kind !== 'fit-bounds') {
      throw new Error('expected a fit-bounds view');
    }
    const latitudeSpan = view.northEast.latitude - view.southWest.latitude;
    const longitudeSpan = view.northEast.longitude - view.southWest.longitude;
    const expectedSpan = 0.01 * (1 + BOUNDS_MARGIN_FRACTION);
    expect(latitudeSpan).toBeCloseTo(expectedSpan, 5);
    expect(longitudeSpan).toBeCloseTo(expectedSpan, 5);
  });

  it('applies a minimum span for a single vertex rather than zooming in absurdly close', () => {
    const vertex = { latitude: 51.5, longitude: -0.1 };

    const view = computeBoundaryMapView([vertex], { latitude: 51.6, longitude: -0.2 });

    if (view.kind !== 'fit-bounds') {
      throw new Error('expected a fit-bounds view');
    }
    expect(view.northEast.latitude - view.southWest.latitude).toBeCloseTo(
      MINIMUM_SPAN_DEGREES,
      6,
    );
    expect(view.northEast.longitude - view.southWest.longitude).toBeCloseTo(
      MINIMUM_SPAN_DEGREES,
      6,
    );
  });

  it('applies a minimum span for a tight cluster of vertices smaller than the minimum', () => {
    const vertices = [
      { latitude: 51.5, longitude: -0.1 },
      { latitude: 51.50002, longitude: -0.10002 },
    ];

    const view = computeBoundaryMapView(vertices, { latitude: 51.6, longitude: -0.2 });

    if (view.kind !== 'fit-bounds') {
      throw new Error('expected a fit-bounds view');
    }
    expect(view.northEast.latitude - view.southWest.latitude).toBeCloseTo(
      MINIMUM_SPAN_DEGREES,
      6,
    );
  });
});
