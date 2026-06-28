import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { MapPage } from '../MapPage';
import { SpyMapPort } from './spies/spy-map-port';
import {
  aZone,
  aSecondZone,
  aBubbleCluster,
  aSinglePinCluster,
  anApplication,
} from './fixtures/map.fixtures';
import { asApplicationUid } from '../../../domain/types';

const h = vi.hoisted(() => {
  const setView = vi.fn();
  return {
    setView,
    mapInstance: {
      getBounds: () => ({
        getWest: () => -0.2,
        getSouth: () => 51.4,
        getEast: () => 0.1,
        getNorth: () => 51.6,
      }),
      getZoom: () => 13,
      setView,
    },
  };
});

vi.mock('react-leaflet', () => ({
  MapContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="map-container">{children}</div>
  ),
  TileLayer: () => <div data-testid="tile-layer" />,
  Marker: ({
    icon,
    eventHandlers,
  }: {
    icon?: { className?: string; html?: string };
    eventHandlers?: { click?: () => void };
  }) => (
    <button
      data-testid="map-marker"
      data-icon-class={icon?.className}
      data-icon-html={icon?.html}
      onClick={() => eventHandlers?.click?.()}
    />
  ),
  useMap: () => h.mapInstance,
  useMapEvents: () => h.mapInstance,
}));

vi.mock('leaflet', () => ({
  default: { divIcon: (opts: unknown) => opts },
  divIcon: (opts: unknown) => opts,
}));

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location">{location.pathname}</div>;
}

function renderMap(spy: SpyMapPort) {
  return render(
    <MemoryRouter initialEntries={['/map']}>
      <Routes>
        <Route path="/map" element={<MapPage port={spy} />} />
        <Route path="/applications/*" element={<LocationProbe />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('MapPage', () => {
  let spy: SpyMapPort;

  beforeEach(() => {
    vi.clearAllMocks();
    spy = new SpyMapPort();
  });

  it('renders the map heading and container once a zone has loaded', async () => {
    spy.fetchMyZonesResult = [aZone()];

    renderMap(spy);

    await waitFor(() => {
      expect(screen.getByTestId('map-container')).toBeInTheDocument();
    });
    expect(screen.getByRole('heading', { name: 'Map' })).toBeInTheDocument();
  });

  it('renders the loading state initially', () => {
    renderMap(spy);
    expect(screen.getByText('Loading...')).toBeInTheDocument();
  });

  it('renders the error state when zone load fails', async () => {
    spy.fetchMyZonesError = new Error('Network unavailable');

    renderMap(spy);

    await waitFor(() => {
      expect(screen.getByText('Network unavailable')).toBeInTheDocument();
    });
  });

  it('requests clusters for the current viewport on mount', async () => {
    spy.fetchMyZonesResult = [aZone()];
    spy.fetchClustersResult = [aBubbleCluster()];

    renderMap(spy);

    await waitFor(() => {
      expect(spy.fetchClustersCalls.length).toBeGreaterThanOrEqual(1);
    });
    const call = spy.fetchClustersCalls[0]!;
    expect(call.bounds).toEqual({ west: -0.2, south: 51.4, east: 0.1, north: 51.6 });
    expect(call.zoom).toBe(13);
    expect(call.status).toBeNull();
  });

  it('renders an amber count bubble for a multi-member cell', async () => {
    spy.fetchMyZonesResult = [aZone()];
    spy.fetchClustersResult = [aBubbleCluster({ count: 9 })];

    renderMap(spy);

    await waitFor(() => {
      const markers = screen.getAllByTestId('map-marker');
      const bubble = markers.find(
        (m) => m.getAttribute('data-icon-class') === 'tc-cluster-bubble-wrapper',
      );
      expect(bubble).toBeDefined();
      expect(bubble!.getAttribute('data-icon-html')).toContain('9');
    });
  });

  it('renders a status-coloured pin for a single-member cell', async () => {
    spy.fetchMyZonesResult = [aZone()];
    spy.fetchClustersResult = [aSinglePinCluster({ statusCounts: { Permitted: 1 } })];

    renderMap(spy);

    await waitFor(() => {
      const markers = screen.getAllByTestId('map-marker');
      const pin = markers.find(
        (m) => m.getAttribute('data-icon-class') === 'tc-status-pin-wrapper',
      );
      expect(pin).toBeDefined();
      expect(pin!.getAttribute('data-icon-html')).toContain('var(--tc-status-permitted)');
    });
  });

  it('zooms in when a multi-member bubble is tapped', async () => {
    spy.fetchMyZonesResult = [aZone()];
    spy.fetchClustersResult = [aBubbleCluster()];

    renderMap(spy);

    let bubble: HTMLElement | undefined;
    await waitFor(() => {
      bubble = screen
        .getAllByTestId('map-marker')
        .find((m) => m.getAttribute('data-icon-class') === 'tc-cluster-bubble-wrapper');
      expect(bubble).toBeDefined();
    });

    fireEvent.click(bubble!);

    expect(h.setView).toHaveBeenCalledTimes(1);
  });

  it('point-reads {authority,name} and opens the app when a single pin is tapped', async () => {
    spy.fetchMyZonesResult = [aZone()];
    const pinCluster = aSinglePinCluster();
    spy.fetchClustersResult = [pinCluster];
    spy.fetchApplicationByMemberResult = anApplication({ uid: asApplicationUid('app-777') });

    renderMap(spy);

    let pin: HTMLElement | undefined;
    await waitFor(() => {
      pin = screen
        .getAllByTestId('map-marker')
        .find((m) => m.getAttribute('data-icon-class') === 'tc-status-pin-wrapper');
      expect(pin).toBeDefined();
    });

    fireEvent.click(pin!);

    await waitFor(() => {
      expect(screen.getByTestId('location')).toHaveTextContent('/applications/app-777');
    });
    expect(spy.fetchApplicationByMemberCalls).toEqual([pinCluster.member]);
  });

  it('refetches clusters with status= when a status chip is selected', async () => {
    spy.fetchMyZonesResult = [aZone()];
    spy.fetchClustersResult = [aBubbleCluster()];

    renderMap(spy);

    await waitFor(() => {
      expect(spy.fetchClustersCalls.length).toBeGreaterThanOrEqual(1);
    });

    fireEvent.click(screen.getByRole('button', { name: 'Granted' }));

    await waitFor(() => {
      expect(spy.fetchClustersCalls.some((c) => c.status === 'Permitted')).toBe(true);
    });
  });

  it('shows a zone picker when more than one zone exists', async () => {
    spy.fetchMyZonesResult = [aZone(), aSecondZone()];

    renderMap(spy);

    await waitFor(() => {
      expect(screen.getByRole('combobox', { name: /watch zone/i })).toBeInTheDocument();
    });
  });

  it('does not show a zone picker for a single zone', async () => {
    spy.fetchMyZonesResult = [aZone()];

    renderMap(spy);

    await waitFor(() => {
      expect(screen.getByTestId('map-container')).toBeInTheDocument();
    });
    expect(screen.queryByRole('combobox', { name: /watch zone/i })).not.toBeInTheDocument();
  });
});
