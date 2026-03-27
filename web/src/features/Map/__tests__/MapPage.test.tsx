import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { MapPage } from '../MapPage';
import { SpyWatchZoneRepository } from './spies/spy-watch-zone-repository';
import { SpyMapApplicationsPort } from './spies/spy-applications-browse-port';
import { aWatchZone, anApplication, aSecondApplication } from './fixtures/map.fixtures';

// Mock react-leaflet — Leaflet cannot render in jsdom (no canvas/DOM layout)
vi.mock('react-leaflet', () => ({
  MapContainer: ({ children, center, zoom }: {
    children: React.ReactNode;
    center: [number, number];
    zoom: number;
  }) => (
    <div data-testid="map-container" data-center={center.join(',')} data-zoom={zoom}>
      {children}
    </div>
  ),
  TileLayer: () => <div data-testid="tile-layer" />,
  Marker: ({ children, position }: {
    children: React.ReactNode;
    position: [number, number];
  }) => (
    <div data-testid="marker" data-position={position.join(',')}>
      {children}
    </div>
  ),
  Popup: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="popup">{children}</div>
  ),
}));

function renderPage(
  watchZoneRepo: SpyWatchZoneRepository,
  applicationsPort: SpyMapApplicationsPort,
) {
  return render(
    <MemoryRouter>
      <MapPage watchZoneRepo={watchZoneRepo} applicationsPort={applicationsPort} />
    </MemoryRouter>,
  );
}

describe('MapPage', () => {
  let zoneSpy: SpyWatchZoneRepository;
  let appsSpy: SpyMapApplicationsPort;

  beforeEach(() => {
    zoneSpy = new SpyWatchZoneRepository();
    appsSpy = new SpyMapApplicationsPort();
  });

  it('renders loading state initially', () => {
    zoneSpy.listResult = [aWatchZone()];
    appsSpy.fetchByAuthorityResults.set(1, []);

    renderPage(zoneSpy, appsSpy);

    expect(screen.getByText('Loading map data...')).toBeInTheDocument();
  });

  it('renders the map container after data loads', async () => {
    zoneSpy.listResult = [aWatchZone()];
    appsSpy.fetchByAuthorityResults.set(1, []);

    renderPage(zoneSpy, appsSpy);

    await waitFor(() => {
      expect(screen.getByTestId('map-container')).toBeInTheDocument();
    });
  });

  it('renders markers for applications with coordinates', async () => {
    zoneSpy.listResult = [aWatchZone()];
    const app1 = anApplication();
    const app2 = aSecondApplication();
    appsSpy.fetchByAuthorityResults.set(1, [app1, app2]);

    renderPage(zoneSpy, appsSpy);

    await waitFor(() => {
      expect(screen.getAllByTestId('marker')).toHaveLength(2);
    });
  });

  it('renders application name in marker popup', async () => {
    zoneSpy.listResult = [aWatchZone()];
    const app = anApplication({ name: '2026/0042/FUL' });
    appsSpy.fetchByAuthorityResults.set(1, [app]);

    renderPage(zoneSpy, appsSpy);

    await waitFor(() => {
      expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument();
    });
  });

  it('renders application address in marker popup', async () => {
    zoneSpy.listResult = [aWatchZone()];
    const app = anApplication({ address: '12 Mill Road, Cambridge, CB1 2AD' });
    appsSpy.fetchByAuthorityResults.set(1, [app]);

    renderPage(zoneSpy, appsSpy);

    await waitFor(() => {
      expect(screen.getByText('12 Mill Road, Cambridge, CB1 2AD')).toBeInTheDocument();
    });
  });

  it('renders a link to the application detail page', async () => {
    zoneSpy.listResult = [aWatchZone()];
    const app = anApplication();
    appsSpy.fetchByAuthorityResults.set(1, [app]);

    renderPage(zoneSpy, appsSpy);

    await waitFor(() => {
      const link = screen.getByRole('link', { name: 'View details' });
      expect(link).toBeInTheDocument();
      expect(link).toHaveAttribute('href', `/applications/${app.uid}`);
    });
  });

  it('renders error message when data fails to load', async () => {
    zoneSpy.listError = new Error('Network unavailable');

    renderPage(zoneSpy, appsSpy);

    await waitFor(() => {
      expect(screen.getByText('Network unavailable')).toBeInTheDocument();
    });
  });

  it('shows empty state when there are no applications', async () => {
    zoneSpy.listResult = [aWatchZone()];
    appsSpy.fetchByAuthorityResults.set(1, []);

    renderPage(zoneSpy, appsSpy);

    await waitFor(() => {
      expect(screen.getByTestId('map-container')).toBeInTheDocument();
    });

    // Map should render even with no markers
    expect(screen.queryAllByTestId('marker')).toHaveLength(0);
  });
});
