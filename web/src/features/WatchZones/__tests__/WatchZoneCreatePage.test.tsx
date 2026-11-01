import React from 'react';
import { render, screen, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { WatchZoneCreatePage } from '../WatchZoneCreatePage';
import { SpyWatchZoneRepository } from './spies/spy-watch-zone-repository';
import { aWatchZone } from './fixtures/watch-zone.fixtures';
import type { GeocodingPort } from '../../../domain/ports/geocoding-port';
import { appStoreUrl } from '../../../config/links';

interface LatLngPayload {
  latlng: { lat: number; lng: number };
}

interface MockMarkerEventHandlers {
  click?: () => void;
}

interface MockMarkerProps {
  position: [number, number];
  eventHandlers?: MockMarkerEventHandlers;
}

interface PathLayerProps {
  positions: [number, number][];
}

const LARGE_ZONE_TITLE = 'Large zones are for notifications, not browsing';

const { capturedMapEvents } = vi.hoisted(() => ({
  capturedMapEvents: { click: undefined as ((event: LatLngPayload) => void) | undefined },
}));

vi.mock('react-leaflet', () => ({
  MapContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="map-container">{children}</div>
  ),
  TileLayer: () => <div data-testid="tile-layer" />,
  Marker: ({ position, eventHandlers }: MockMarkerProps) => (
    <div data-testid="map-marker" data-lat={position[0]} data-lng={position[1]}>
      {eventHandlers?.click && (
        <button type="button" data-testid="marker-click" onClick={eventHandlers.click}>
          marker
        </button>
      )}
    </div>
  ),
  Circle: () => <div data-testid="map-circle" />,
  Polygon: ({ positions }: PathLayerProps) => (
    <div data-testid="boundary-polygon" data-count={positions.length} />
  ),
  Polyline: ({ positions }: PathLayerProps) => (
    <div data-testid="boundary-polyline" data-count={positions.length} />
  ),
  useMap: () => ({
    fitBounds: vi.fn(),
  }),
  useMapEvents: (handlers: { click?: (event: LatLngPayload) => void }) => {
    capturedMapEvents.click = handlers.click;
    return null;
  },
}));

vi.mock('leaflet', () => ({
  default: {
    icon: () => ({}),
    Icon: { Default: { mergeOptions: () => {} } },
    latLng: () => ({ toBounds: () => ({ pad: () => ({}) }) }),
  },
  icon: () => ({}),
  Icon: { Default: { mergeOptions: () => {} } },
  latLng: () => ({ toBounds: () => ({ pad: () => ({}) }) }),
}));

class SpyGeocodingPort implements GeocodingPort {
  geocodeCalls: string[] = [];
  geocodeResult = { latitude: 52.2053, longitude: 0.1218 };
  geocodeError: Error | null = null;

  async geocode(postcode: string) {
    this.geocodeCalls.push(postcode);
    if (this.geocodeError) {
      throw this.geocodeError;
    }
    return this.geocodeResult;
  }
}

function renderWithRouter(ui: React.ReactElement) {
  return render(<MemoryRouter>{ui}</MemoryRouter>);
}

describe('WatchZoneCreatePage', () => {
  let repoSpy: SpyWatchZoneRepository;
  let geocodingSpy: SpyGeocodingPort;
  let navigatedTo: string | null;

  function navigate(path: string) {
    navigatedTo = path;
  }

  beforeEach(() => {
    repoSpy = new SpyWatchZoneRepository();
    geocodingSpy = new SpyGeocodingPort();
    navigatedTo = null;
  });

  it('renders the postcode step initially', () => {
    renderWithRouter(
      <WatchZoneCreatePage
        repository={repoSpy}
        geocodingPort={geocodingSpy}
        navigate={navigate}
      />,
    );

    expect(screen.getByText(/create watch zone/i)).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: /postcode/i })).toBeInTheDocument();
  });

  it('advances to details step after postcode lookup', async () => {
    const user = userEvent.setup();

    renderWithRouter(
      <WatchZoneCreatePage
        repository={repoSpy}
        geocodingPort={geocodingSpy}
        navigate={navigate}
      />,
    );

    const postcodeInput = screen.getByRole('textbox', { name: /postcode/i });
    await user.type(postcodeInput, 'CB1 2AD');
    await user.click(screen.getByRole('button', { name: /look up/i }));

    expect(await screen.findByLabelText(/zone name/i)).toBeInTheDocument();
    expect(screen.getByRole('radiogroup', { name: /radius/i })).toBeInTheDocument();
  });

  it('saves zone with form data', async () => {
    const user = userEvent.setup();
    repoSpy.createResult = aWatchZone();

    renderWithRouter(
      <WatchZoneCreatePage
        repository={repoSpy}
        geocodingPort={geocodingSpy}
        navigate={navigate}
      />,
    );

    await user.type(screen.getByRole('textbox', { name: /postcode/i }), 'CB1 2AD');
    await user.click(screen.getByRole('button', { name: /look up/i }));

    const nameInput = await screen.findByLabelText(/zone name/i);
    await user.type(nameInput, 'Home');
    await user.click(screen.getByLabelText('5 km'));

    await user.click(screen.getByRole('button', { name: /next/i }));

    expect(screen.getByTestId('map-container')).toBeInTheDocument();
    expect(screen.getByText('CB1 2AD')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /confirm/i }));

    expect(repoSpy.createCalls).toHaveLength(1);
    expect(repoSpy.createCalls[0]?.name).toBe('Home');
    expect(repoSpy.createCalls[0]?.radiusMetres).toBe(5000);
    expect(navigatedTo).toBe('/watch-zones');
  });

  it('shows error when save fails', async () => {
    const user = userEvent.setup();
    repoSpy.createError = new Error('Create failed');

    renderWithRouter(
      <WatchZoneCreatePage
        repository={repoSpy}
        geocodingPort={geocodingSpy}
        navigate={navigate}
      />,
    );

    await user.type(screen.getByRole('textbox', { name: /postcode/i }), 'CB1 2AD');
    await user.click(screen.getByRole('button', { name: /look up/i }));

    const nameInput = await screen.findByLabelText(/zone name/i);
    await user.type(nameInput, 'Home');

    await user.click(screen.getByRole('button', { name: /next/i }));
    await user.click(screen.getByRole('button', { name: /confirm/i }));

    expect(await screen.findByText('Create failed')).toBeInTheDocument();
    expect(navigatedTo).toBeNull();
  });

  it('hides the large-zone warning at the default 2km radius', async () => {
    const user = userEvent.setup();

    renderWithRouter(
      <WatchZoneCreatePage
        repository={repoSpy}
        geocodingPort={geocodingSpy}
        navigate={navigate}
      />,
    );

    await user.type(screen.getByRole('textbox', { name: /postcode/i }), 'CB1 2AD');
    await user.click(screen.getByRole('button', { name: /look up/i }));

    await screen.findByLabelText(/zone name/i);

    expect(screen.queryByText(LARGE_ZONE_TITLE)).not.toBeInTheDocument();
  });

  it('shows the large-zone warning when a radius above 2km is selected and hides it again', async () => {
    const user = userEvent.setup();

    renderWithRouter(
      <WatchZoneCreatePage
        repository={repoSpy}
        geocodingPort={geocodingSpy}
        navigate={navigate}
      />,
    );

    await user.type(screen.getByRole('textbox', { name: /postcode/i }), 'CB1 2AD');
    await user.click(screen.getByRole('button', { name: /look up/i }));
    await screen.findByLabelText(/zone name/i);

    await user.click(screen.getByLabelText('5 km'));
    expect(screen.getByText(LARGE_ZONE_TITLE)).toBeInTheDocument();

    await user.click(screen.getByLabelText('1 km'));
    expect(screen.queryByText(LARGE_ZONE_TITLE)).not.toBeInTheDocument();
  });

  it('shows the large-zone warning live while a custom shape over the limit is drawn', async () => {
    const user = userEvent.setup();

    renderWithRouter(
      <WatchZoneCreatePage
        repository={repoSpy}
        geocodingPort={geocodingSpy}
        navigate={navigate}
        tier="Personal"
      />,
    );

    await user.type(screen.getByRole('textbox', { name: /postcode/i }), 'CB1 2AD');
    await user.click(screen.getByRole('button', { name: /look up/i }));
    await screen.findByLabelText(/zone name/i);

    await user.click(screen.getByLabelText('5 km'));
    await user.click(screen.getByRole('radio', { name: /^custom shape$/i }));
    expect(screen.queryByText(LARGE_ZONE_TITLE)).not.toBeInTheDocument();

    act(() => {
      capturedMapEvents.click?.({ latlng: { lat: 51.95, lng: -1.05 } });
    });
    act(() => {
      capturedMapEvents.click?.({ latlng: { lat: 51.95, lng: -0.95 } });
    });
    act(() => {
      capturedMapEvents.click?.({ latlng: { lat: 52.05, lng: -0.95 } });
    });

    expect(screen.getByText(LARGE_ZONE_TITLE)).toBeInTheDocument();
  });

  it('has a cancel link back to the list', () => {
    renderWithRouter(
      <WatchZoneCreatePage
        repository={repoSpy}
        geocodingPort={geocodingSpy}
        navigate={navigate}
      />,
    );

    const cancelLink = screen.getByRole('link', { name: /cancel/i });
    expect(cancelLink).toHaveAttribute('href', '/watch-zones');
  });

  describe('shape mode', () => {
    async function reachDetailsStep(tier?: 'Free' | 'Personal' | 'Pro') {
      const user = userEvent.setup();
      renderWithRouter(
        <WatchZoneCreatePage
          repository={repoSpy}
          geocodingPort={geocodingSpy}
          navigate={navigate}
          tier={tier}
        />,
      );

      await user.type(screen.getByRole('textbox', { name: /postcode/i }), 'CB1 2AD');
      await user.click(screen.getByRole('button', { name: /look up/i }));
      await screen.findByLabelText(/zone name/i);

      return user;
    }

    it('shows RadiusPicker and the custom-shape upsell, with no shape toggle, for Free tier', async () => {
      await reachDetailsStep('Free');

      expect(screen.getByRole('radiogroup', { name: /radius/i })).toBeInTheDocument();
      expect(screen.queryByRole('radiogroup', { name: /shape/i })).not.toBeInTheDocument();
      expect(
        screen.getByRole('heading', { name: /draw any shape/i }),
      ).toBeInTheDocument();

      const cta = screen.getByRole('link', { name: /upgrade/i });
      expect(cta).toHaveAttribute(
        'href',
        appStoreUrl('web-watch-zone-create-custom-shape'),
      );
      expect(cta).toHaveAttribute('target', '_blank');
      expect(cta).toHaveAttribute('rel', 'noopener noreferrer');
    });

    it('shows the shape toggle for Personal/Pro tier', async () => {
      await reachDetailsStep('Pro');

      expect(screen.getByRole('radiogroup', { name: /shape/i })).toBeInTheDocument();
      // Defaults to circle mode, so the radius picker is still visible.
      expect(screen.getByRole('radiogroup', { name: /radius/i })).toBeInTheDocument();
      expect(
        screen.queryByRole('heading', { name: /draw any shape/i }),
      ).not.toBeInTheDocument();
    });

    it('switching to custom shape hides the radius picker and shows the drawing map', async () => {
      const user = await reachDetailsStep('Personal');

      await user.click(screen.getByRole('radio', { name: /^custom shape$/i }));

      expect(screen.queryByRole('radiogroup', { name: /radius/i })).not.toBeInTheDocument();
      expect(screen.getByTestId('map-container')).toBeInTheDocument();
    });

    it('undoes the last misplaced vertex without losing the earlier ones', async () => {
      const user = await reachDetailsStep('Personal');

      await user.click(screen.getByRole('radio', { name: /^custom shape$/i }));

      act(() => {
        capturedMapEvents.click?.({ latlng: { lat: 52.2, lng: 0.1 } });
      });
      act(() => {
        capturedMapEvents.click?.({ latlng: { lat: 52.2, lng: 0.12 } });
      });

      expect(screen.getAllByTestId('map-marker')).toHaveLength(2);

      await user.click(screen.getByRole('button', { name: 'Undo' }));

      expect(screen.getAllByTestId('map-marker')).toHaveLength(1);
    });

    it('draws a polygon, closes it, and saves the zone with the boundary', async () => {
      const user = await reachDetailsStep('Pro');
      repoSpy.createResult = aWatchZone();

      await user.type(screen.getByLabelText(/zone name/i), 'Home');
      await user.click(screen.getByRole('radio', { name: /^custom shape$/i }));

      act(() => {
        capturedMapEvents.click?.({ latlng: { lat: 52.2, lng: 0.1 } });
      });
      act(() => {
        capturedMapEvents.click?.({ latlng: { lat: 52.2, lng: 0.12 } });
      });
      act(() => {
        capturedMapEvents.click?.({ latlng: { lat: 52.21, lng: 0.12 } });
      });

      const firstVertexClose = screen.getAllByTestId('marker-click')[0];
      await user.click(firstVertexClose!);

      expect(screen.getByTestId('boundary-polygon')).toBeInTheDocument();

      await user.click(screen.getByRole('button', { name: /next/i }));
      await user.click(screen.getByRole('button', { name: /confirm/i }));

      expect(repoSpy.createCalls).toHaveLength(1);
      expect(repoSpy.createCalls[0]?.boundary).toEqual({
        type: 'Polygon',
        coordinates: [
          [
            [0.1, 52.2],
            [0.12, 52.2],
            [0.12, 52.21],
            [0.1, 52.2],
          ],
        ],
      });
      expect(navigatedTo).toBe('/watch-zones');
    });
  });
});
