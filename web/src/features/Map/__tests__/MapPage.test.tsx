import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { MapPage } from '../MapPage';
import { SpyMapPort } from './spies/spy-map-port';
import { anAuthority, anApplication, aSavedApplication } from './fixtures/map.fixtures';
import { asApplicationUid } from '../../../domain/types';

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
  useMap: () => ({ fitBounds: vi.fn() }),
}));

vi.mock('leaflet', () => ({
  default: {
    divIcon: () => ({}),
    latLngBounds: () => ({}),
    latLng: (lat: number, lng: number) => ({ lat, lng }),
  },
  divIcon: () => ({}),
  latLngBounds: () => ({}),
  latLng: (lat: number, lng: number) => ({ lat, lng }),
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

  it('renders "Save application" button for unsaved apps', async () => {
    const auth = anAuthority();
    spy.fetchMyAuthoritiesResult = [auth];
    spy.fetchApplicationsByAuthorityResults.set(auth.id as number, [anApplication()]);
    spy.fetchSavedApplicationsResult = [];

    render(
      <MemoryRouter>
        <MapPage port={spy} />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Save application' })).toBeInTheDocument();
    });
  });

  it('renders "Unsave application" button for saved apps', async () => {
    const auth = anAuthority();
    spy.fetchMyAuthoritiesResult = [auth];
    spy.fetchApplicationsByAuthorityResults.set(auth.id as number, [anApplication()]);
    spy.fetchSavedApplicationsResult = [aSavedApplication()];

    render(
      <MemoryRouter>
        <MapPage port={spy} />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Unsave application' })).toBeInTheDocument();
    });
  });

  it('calls saveApplication when save button is clicked', async () => {
    const auth = anAuthority();
    spy.fetchMyAuthoritiesResult = [auth];
    spy.fetchApplicationsByAuthorityResults.set(auth.id as number, [anApplication()]);
    spy.fetchSavedApplicationsResult = [];

    render(
      <MemoryRouter>
        <MapPage port={spy} />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Save application' })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Save application' }));

    await waitFor(() => {
      expect(spy.saveApplicationCalls).toEqual([asApplicationUid('app-001')]);
    });
  });

  it('calls unsaveApplication when unsave button is clicked', async () => {
    const auth = anAuthority();
    spy.fetchMyAuthoritiesResult = [auth];
    spy.fetchApplicationsByAuthorityResults.set(auth.id as number, [anApplication()]);
    spy.fetchSavedApplicationsResult = [aSavedApplication()];

    render(
      <MemoryRouter>
        <MapPage port={spy} />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Unsave application' })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Unsave application' }));

    await waitFor(() => {
      expect(spy.unsaveApplicationCalls).toEqual([asApplicationUid('app-001')]);
    });
  });
});
