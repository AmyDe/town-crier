import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { ConfirmMap } from '../ConfirmMap';

vi.mock('react-leaflet', () => ({
  MapContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="map-container">{children}</div>
  ),
  TileLayer: () => <div data-testid="tile-layer" />,
  Marker: () => <div data-testid="map-marker" />,
  Circle: () => <div data-testid="map-circle" />,
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
});
