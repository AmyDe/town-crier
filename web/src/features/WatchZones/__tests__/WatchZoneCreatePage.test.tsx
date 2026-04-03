import React from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { WatchZoneCreatePage } from '../WatchZoneCreatePage';
import { SpyWatchZoneRepository } from './spies/spy-watch-zone-repository';
import { aWatchZone } from './fixtures/watch-zone.fixtures';
import type { GeocodingPort } from '../../../domain/ports/geocoding-port';

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

    // Should now see details form
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

    // Step 1: Look up postcode
    await user.type(screen.getByRole('textbox', { name: /postcode/i }), 'CB1 2AD');
    await user.click(screen.getByRole('button', { name: /look up/i }));

    // Step 2: Fill in details
    const nameInput = await screen.findByLabelText(/zone name/i);
    await user.type(nameInput, 'Home');
    await user.click(screen.getByLabelText('5 km'));

    // Advance to confirm step
    await user.click(screen.getByRole('button', { name: /next/i }));

    // Step 3: Confirm — should see map and postcode
    expect(screen.getByTestId('map-container')).toBeInTheDocument();
    expect(screen.getByText('CB1 2AD')).toBeInTheDocument();

    // Confirm
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
});
