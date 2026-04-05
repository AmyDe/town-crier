import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { MapPage } from '../MapPage';
import { SpyMapPort } from './spies/spy-map-port';
import { anAuthority, anApplication } from './fixtures/map.fixtures';

// Mock react-leaflet — Leaflet doesn't render in jsdom
vi.mock('react-leaflet', () => ({
  MapContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="map-container">{children}</div>
  ),
  TileLayer: () => <div data-testid="tile-layer" />,
  Marker: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="map-marker">{children}</div>
  ),
  Popup: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="map-popup">{children}</div>
  ),
}));

// Mock leaflet itself
vi.mock('leaflet', () => ({
  default: {
    icon: () => ({}),
    Icon: { Default: { mergeOptions: () => {} } },
  },
  icon: () => ({}),
  Icon: { Default: { mergeOptions: () => {} } },
}));

describe('MapPage', () => {
  let spy: SpyMapPort;

  beforeEach(() => {
    spy = new SpyMapPort();
  });

  it('renders map heading and container', async () => {
    spy.fetchMyAuthoritiesResult = [anAuthority()];

    render(
      <MemoryRouter>
        <MapPage port={spy} />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('map-container')).toBeInTheDocument();
    });

    expect(screen.getByRole('heading', { name: 'Map' })).toBeInTheDocument();
  });

  it('renders loading state initially', () => {
    render(
      <MemoryRouter>
        <MapPage port={spy} />
      </MemoryRouter>,
    );

    expect(screen.getByText('Loading...')).toBeInTheDocument();
  });

  it('renders error state on failure', async () => {
    spy.fetchMyAuthoritiesError = new Error('Network unavailable');

    render(
      <MemoryRouter>
        <MapPage port={spy} />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText('Network unavailable')).toBeInTheDocument();
    });
  });

  it('renders application markers with popups showing summary info', async () => {
    const auth = anAuthority();
    const app = anApplication();
    spy.fetchMyAuthoritiesResult = [auth];
    spy.fetchApplicationsByAuthorityResults.set(auth.id as number, [app]);

    render(
      <MemoryRouter>
        <MapPage port={spy} />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('map-marker')).toBeInTheDocument();
    });

    expect(screen.getByText('Erection of two-storey rear extension')).toBeInTheDocument();
    expect(screen.getByText('12 Mill Road, Cambridge')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /View details/i })).toHaveAttribute(
      'href',
      '/applications/app-001',
    );
  });

  it('skips applications without coordinates', async () => {
    const auth = anAuthority();
    const appWithCoords = anApplication();
    const appWithoutCoords = anApplication({
      uid: 'no-coords' as never,
      latitude: null,
      longitude: null,
    });
    spy.fetchMyAuthoritiesResult = [auth];
    spy.fetchApplicationsByAuthorityResults.set(auth.id as number, [
      appWithCoords,
      appWithoutCoords,
    ]);

    render(
      <MemoryRouter>
        <MapPage port={spy} />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getAllByTestId('map-marker')).toHaveLength(1);
    });
  });
});
