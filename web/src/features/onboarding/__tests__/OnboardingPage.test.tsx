import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route } from 'react-router';
import { describe, it, expect, vi } from 'vitest';
import { OnboardingPage } from '../OnboardingPage';
import { SpyOnboardingPort } from './spies/spy-onboarding-port';
import { SpyGeocodingPort } from './spies/spy-geocoding-port';
import { appStoreUrl } from '../../../config/links';

vi.mock('react-leaflet', () => ({
  MapContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="map-container">{children}</div>
  ),
  TileLayer: () => <div data-testid="tile-layer" />,
  Marker: () => <div data-testid="map-marker" />,
  Circle: () => <div data-testid="map-circle" />,
  Polygon: () => <div data-testid="boundary-polygon" />,
  Polyline: () => <div data-testid="boundary-polyline" />,
  useMap: () => ({
    fitBounds: vi.fn(),
  }),
  useMapEvents: () => null,
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

function renderOnboarding(
  onboardingPort: SpyOnboardingPort,
  geocodingPort: SpyGeocodingPort,
) {
  return render(
    <MemoryRouter initialEntries={['/onboarding']}>
      <Routes>
        <Route
          path="/onboarding"
          element={
            <OnboardingPage
              onboardingPort={onboardingPort}
              geocodingPort={geocodingPort}
            />
          }
        />
        <Route path="/dashboard" element={<div>Dashboard</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('OnboardingPage', () => {
  it('renders the welcome step initially', () => {
    renderOnboarding(new SpyOnboardingPort(), new SpyGeocodingPort());

    expect(screen.getByRole('heading', { name: /welcome/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /get started/i })).toBeInTheDocument();
  });

  it('advances to postcode step on Get Started click', async () => {
    const user = userEvent.setup();
    renderOnboarding(new SpyOnboardingPort(), new SpyGeocodingPort());

    await user.click(screen.getByRole('button', { name: /get started/i }));

    expect(screen.getByLabelText(/postcode/i)).toBeInTheDocument();
  });

  it('advances to radius step after postcode geocode', async () => {
    const user = userEvent.setup();
    const geocodingSpy = new SpyGeocodingPort();
    renderOnboarding(new SpyOnboardingPort(), geocodingSpy);

    await user.click(screen.getByRole('button', { name: /get started/i }));
    await user.type(screen.getByLabelText(/postcode/i), 'SW1A 1AA');
    await user.click(screen.getByRole('button', { name: /look up/i }));

    await waitFor(() => {
      expect(screen.getByRole('radiogroup', { name: /radius/i })).toBeInTheDocument();
    });
  });

  it('advances to confirm step after radius selection', async () => {
    const user = userEvent.setup();
    const geocodingSpy = new SpyGeocodingPort();
    renderOnboarding(new SpyOnboardingPort(), geocodingSpy);

    // Go through welcome → postcode → radius
    await user.click(screen.getByRole('button', { name: /get started/i }));
    await user.type(screen.getByLabelText(/postcode/i), 'SW1A 1AA');
    await user.click(screen.getByRole('button', { name: /look up/i }));

    await waitFor(() => {
      expect(screen.getByRole('radiogroup', { name: /radius/i })).toBeInTheDocument();
    });

    // Select 5km and confirm
    await user.click(screen.getByLabelText('5 km'));
    await user.click(screen.getByRole('button', { name: /next/i }));

    expect(screen.getByRole('button', { name: /confirm/i })).toBeInTheDocument();
  });

  it('shows postcode and map on confirm step', async () => {
    const user = userEvent.setup();
    const geocodingSpy = new SpyGeocodingPort();
    renderOnboarding(new SpyOnboardingPort(), geocodingSpy);

    await user.click(screen.getByRole('button', { name: /get started/i }));
    await user.type(screen.getByLabelText(/postcode/i), 'SW1A 1AA');
    await user.click(screen.getByRole('button', { name: /look up/i }));

    await waitFor(() => {
      expect(screen.getByRole('radiogroup', { name: /radius/i })).toBeInTheDocument();
    });

    await user.click(screen.getByLabelText('2 km'));
    await user.click(screen.getByRole('button', { name: /next/i }));

    expect(screen.getByTestId('map-container')).toBeInTheDocument();
    expect(screen.getByText('SW1A 1AA')).toBeInTheDocument();
  });

  it('calls APIs and navigates to dashboard on confirm', async () => {
    const user = userEvent.setup();
    const onboardingSpy = new SpyOnboardingPort();
    const geocodingSpy = new SpyGeocodingPort();
    geocodingSpy.geocodeResult = { latitude: 51.5074, longitude: -0.1278 };
    renderOnboarding(onboardingSpy, geocodingSpy);

    // Complete flow: welcome → postcode → radius → confirm
    await user.click(screen.getByRole('button', { name: /get started/i }));
    await user.type(screen.getByLabelText(/postcode/i), 'SW1A 1AA');
    await user.click(screen.getByRole('button', { name: /look up/i }));

    await waitFor(() => {
      expect(screen.getByRole('radiogroup', { name: /radius/i })).toBeInTheDocument();
    });

    await user.click(screen.getByLabelText('2 km'));
    await user.click(screen.getByRole('button', { name: /next/i }));
    await user.click(screen.getByRole('button', { name: /confirm/i }));

    await waitFor(() => {
      expect(screen.getByText('Dashboard')).toBeInTheDocument();
    });

    expect(onboardingSpy.createProfileCalls).toBe(1);
    expect(onboardingSpy.createWatchZoneCalls).toHaveLength(1);
    expect(onboardingSpy.createWatchZoneCalls[0]).toEqual({
      name: 'Home',
      latitude: 51.5074,
      longitude: -0.1278,
      radiusMetres: 2000,
    });
  });

  it('shows the custom-shape upsell alongside the radius picker for a Free-tier user', async () => {
    const user = userEvent.setup();
    const geocodingSpy = new SpyGeocodingPort();
    renderOnboarding(new SpyOnboardingPort(), geocodingSpy);

    await user.click(screen.getByRole('button', { name: /get started/i }));
    await user.type(screen.getByLabelText(/postcode/i), 'SW1A 1AA');
    await user.click(screen.getByRole('button', { name: /look up/i }));

    await waitFor(() => {
      expect(screen.getByRole('radiogroup', { name: /radius/i })).toBeInTheDocument();
    });

    expect(
      screen.getByRole('heading', { name: /draw any shape/i }),
    ).toBeInTheDocument();
    // No web checkout exists (GH#1031) — the upsell CTA links out to the App
    // Store instead of a web purchase flow, tagged so installs attribute
    // back to this surface.
    const cta = screen.getByRole('link', { name: /upgrade/i });
    expect(cta).toHaveAttribute('href', appStoreUrl('web-onboarding-custom-shape'));
    expect(cta).toHaveAttribute('target', '_blank');
    expect(cta).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('shows error when API call fails', async () => {
    const user = userEvent.setup();
    const onboardingSpy = new SpyOnboardingPort();
    onboardingSpy.createProfileError = new Error('Server error');
    const geocodingSpy = new SpyGeocodingPort();
    renderOnboarding(onboardingSpy, geocodingSpy);

    // Complete flow up to confirm
    await user.click(screen.getByRole('button', { name: /get started/i }));
    await user.type(screen.getByLabelText(/postcode/i), 'SW1A 1AA');
    await user.click(screen.getByRole('button', { name: /look up/i }));

    await waitFor(() => {
      expect(screen.getByRole('radiogroup', { name: /radius/i })).toBeInTheDocument();
    });

    await user.click(screen.getByLabelText('1 km'));
    await user.click(screen.getByRole('button', { name: /next/i }));
    await user.click(screen.getByRole('button', { name: /confirm/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Server error');
    });
  });
});
