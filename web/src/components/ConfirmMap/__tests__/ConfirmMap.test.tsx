import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { ConfirmMap } from '../ConfirmMap';

interface CirclePathOptions {
  color: string;
  fillColor: string;
  fillOpacity: number;
  weight: number;
  dashArray?: string;
}

let lastCirclePathOptions: CirclePathOptions | null = null;

vi.mock('react-leaflet', () => ({
  MapContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="map-container">{children}</div>
  ),
  TileLayer: () => <div data-testid="tile-layer" />,
  Marker: () => <div data-testid="map-marker" />,
  Circle: ({ pathOptions }: { pathOptions: CirclePathOptions }) => {
    lastCirclePathOptions = pathOptions;
    return <div data-testid="map-circle" />;
  },
  useMap: () => ({
    fitBounds: vi.fn(),
  }),
}));

const fakeBounds = { pad: () => fakeBounds };

vi.mock('leaflet', () => ({
  default: {
    icon: () => ({}),
    Icon: { Default: { mergeOptions: () => {} } },
    latLng: () => ({ toBounds: () => fakeBounds }),
  },
  icon: () => ({}),
  Icon: { Default: { mergeOptions: () => {} } },
  latLng: () => ({ toBounds: () => fakeBounds }),
}));

describe('ConfirmMap', () => {
  it('renders map container with marker and circle', () => {
    render(
      <ConfirmMap latitude={51.5074} longitude={-0.1278} radiusMetres={2000} />,
    );

    expect(screen.getByTestId('map-container')).toBeInTheDocument();
    expect(screen.getByTestId('tile-layer')).toBeInTheDocument();
    expect(screen.getByTestId('map-marker')).toBeInTheDocument();
    expect(screen.getByTestId('map-circle')).toBeInTheDocument();
  });

  it('draws the zone circle as a dashed amber ring at low fill opacity', () => {
    render(
      <ConfirmMap latitude={51.5074} longitude={-0.1278} radiusMetres={2000} />,
    );

    expect(lastCirclePathOptions).not.toBeNull();
    // v2 amber (#E9A620) — Leaflet SVG layers can't consume CSS custom
    // properties, so this is kept in sync with tokens.css by hand.
    expect(lastCirclePathOptions!.color).toContain('233, 166, 32');
    expect(lastCirclePathOptions!.fillColor).toContain('233, 166, 32');
    expect(lastCirclePathOptions!.fillOpacity).toBeLessThanOrEqual(0.15);
    expect(lastCirclePathOptions!.dashArray).toBeDefined();
  });
});
